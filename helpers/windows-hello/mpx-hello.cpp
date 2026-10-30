// mpx-hello.exe: asks Windows Hello (fingerprint, face or PIN) to confirm that
// a human is present. Used by the Multiplex daemon for Layer 5 approvals
// (docs/04-safety.md, capability check CC-05).
//
//   mpx-hello.exe check
//   mpx-hello.exe verify --message "<text>" [--timeout <seconds>]
//
// Prints exactly one JSON line on stdout: {"result":"...","detail":"..."}.
// Exit codes: 0 verified/available, 1 canceled, 2 not available / not
// configured / disabled / busy, 3 error (including retries exhausted),
// 4 timeout. It never reads stdin and parses nothing but its own arguments.

#include <windows.h>
#include <unknwn.h>

#include <winrt/base.h>
#include <winrt/Windows.Foundation.h>
#include <winrt/Windows.Security.Credentials.UI.h>

#include <UserConsentVerifierInterop.h>

#include <cstdio>
#include <cwchar>
#include <string>

using namespace winrt;
using namespace Windows::Foundation;
using namespace Windows::Security::Credentials::UI;

namespace {

// Output -----------------------------------------------------------------

std::string ToUtf8(const std::wstring& w) {
    if (w.empty()) return {};
    int n = WideCharToMultiByte(CP_UTF8, 0, w.data(), (int)w.size(), nullptr, 0, nullptr, nullptr);
    std::string s(n, '\0');
    WideCharToMultiByte(CP_UTF8, 0, w.data(), (int)w.size(), s.data(), n, nullptr, nullptr);
    return s;
}

std::string JsonEscape(const std::string& s) {
    std::string o;
    for (unsigned char c : s) {
        switch (c) {
        case '"': o += "\\\""; break;
        case '\\': o += "\\\\"; break;
        case '\n': o += "\\n"; break;
        case '\r': o += "\\r"; break;
        case '\t': o += "\\t"; break;
        default:
            if (c < 0x20) { char b[8]; std::snprintf(b, sizeof b, "\\u%04x", c); o += b; }
            else o += (char)c;
        }
    }
    return o;
}

// Emit(): the single JSON line the Go side parses, then return the exit code.
int Emit(const char* result, const std::string& detail, int code) {
    std::printf("{\"result\":\"%s\",\"detail\":\"%s\"}\n", result, JsonEscape(detail).c_str());
    std::fflush(stdout);
    return code;
}

// Owner window -----------------------------------------------------------
//
// RequestVerificationAsync(message) has no owner window, so from a console
// process the Hello dialog often opens BEHIND the terminal (or never gets
// focus). The fix is the interop interface, which takes an HWND to parent to.
//
// Which HWND? GetConsoleWindow() is right for classic conhost, but under
// Windows Terminal / VS Code (ConPTY) it returns a hidden pseudo-console
// window; parenting to an invisible window gives the same problem. So: use
// the console window only if it is visible, otherwise create our own tiny,
// fully transparent, topmost tool window (no taskbar button) and make it the
// foreground window. A foreground owner makes Windows put the dialog in front.

LRESULT CALLBACK OwnerProc(HWND h, UINT m, WPARAM w, LPARAM l) { return DefWindowProcW(h, m, w, l); }

HWND CreateOwnerWindow() {
    WNDCLASSEXW wc{sizeof wc};
    wc.lpfnWndProc = OwnerProc;
    wc.hInstance = GetModuleHandleW(nullptr);
    wc.lpszClassName = L"MpxHelloOwner";
    RegisterClassExW(&wc);
    int x = GetSystemMetrics(SM_CXSCREEN) / 2, y = GetSystemMetrics(SM_CYSCREEN) / 2;
    HWND h = CreateWindowExW(WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_LAYERED, wc.lpszClassName,
                             L"Multiplex approval", WS_POPUP, x, y, 1, 1, nullptr, nullptr, wc.hInstance, nullptr);
    if (!h) return nullptr;
    SetLayeredWindowAttributes(h, 0, 0, LWA_ALPHA);  // alpha 0: visible to the system, invisible to the eye
    ShowWindow(h, SW_SHOW);
    return h;
}

HWND PickOwner(HWND* created) {
    HWND con = GetConsoleWindow();
    if (con && IsWindowVisible(con)) return con;
    *created = CreateOwnerWindow();
    return *created;
}

// Best effort: Windows only lets the foreground process hand focus on, so
// these may be refused. The dialog is still shown, just possibly not in front.
void Foreground(HWND h) {
    if (!h) return;
    AllowSetForegroundWindow(ASFW_ANY);
    BringWindowToTop(h);
    SetForegroundWindow(h);
}

// Mapping ----------------------------------------------------------------

int FromAvailability(UserConsentVerifierAvailability a, bool forCheck) {
    switch (a) {
    case UserConsentVerifierAvailability::Available:
        return Emit(forCheck ? "available" : "verified", "Windows Hello is available", 0);
    case UserConsentVerifierAvailability::DeviceNotPresent:
        return Emit("not_available", "no Windows Hello capable device", 2);
    case UserConsentVerifierAvailability::NotConfiguredForUser:
        return Emit("not_configured", "Windows Hello is not set up for this user", 2);
    case UserConsentVerifierAvailability::DisabledByPolicy:
        return Emit("disabled_by_policy", "Windows Hello is disabled by policy", 2);
    case UserConsentVerifierAvailability::DeviceBusy:
        return Emit("device_busy", "Windows Hello device is busy", 2);
    }
    return Emit("error", "unknown availability value", 3);
}

int FromResult(UserConsentVerificationResult r) {
    switch (r) {
    case UserConsentVerificationResult::Verified: return Emit("verified", "", 0);
    case UserConsentVerificationResult::Canceled: return Emit("canceled", "the user canceled", 1);
    case UserConsentVerificationResult::DeviceNotPresent: return Emit("not_available", "no Windows Hello capable device", 2);
    case UserConsentVerificationResult::NotConfiguredForUser: return Emit("not_configured", "Windows Hello is not set up for this user", 2);
    case UserConsentVerificationResult::DisabledByPolicy: return Emit("disabled_by_policy", "Windows Hello is disabled by policy", 2);
    case UserConsentVerificationResult::DeviceBusy: return Emit("device_busy", "Windows Hello device is busy", 2);
    case UserConsentVerificationResult::RetriesExhausted: return Emit("retries_exhausted", "too many failed attempts", 3);
    }
    return Emit("error", "unknown verification result", 3);
}

// Commands ---------------------------------------------------------------

int Check() {
    auto a = UserConsentVerifier::CheckAvailabilityAsync().get();  // MTA: blocking get() is allowed
    return FromAvailability(a, true);
}

int Verify(std::wstring message, unsigned seconds) {
    // Unknown limits on the dialog text; keep it short and let the daemon send the detail to the TUI.
    if (message.size() > 256) { message.resize(255); message += L"…"; }

    HWND created = nullptr;
    HWND owner = PickOwner(&created);
    Foreground(owner);

    auto interop = get_activation_factory<UserConsentVerifier, IUserConsentVerifierInterop>();
    IAsyncOperation<UserConsentVerificationResult> op{nullptr};
    hstring msg{message};
    check_hresult(interop->RequestVerificationForWindowAsync(
        owner, static_cast<HSTRING>(get_abi(msg)),
        guid_of<IAsyncOperation<UserConsentVerificationResult>>(), put_abi(op)));

    // Wait with a message pump, not op.get(): the owner window lives on this
    // thread and must stay responsive (else Windows ghosts it as "not
    // responding"). Completed() posts a thread message to wake GetMessage;
    // a thread timer gives the timeout.
    MSG m;
    PeekMessageW(&m, nullptr, WM_USER, WM_USER, PM_NOREMOVE);  // make sure the queue exists
    DWORD tid = GetCurrentThreadId();
    op.Completed([tid](auto const&, AsyncStatus) { PostThreadMessageW(tid, WM_APP, 0, 0); });
    UINT_PTR timer = SetTimer(nullptr, 0, seconds * 1000, nullptr);
    bool timedOut = false;
    while (op.Status() == AsyncStatus::Started) {
        if (GetMessageW(&m, nullptr, 0, 0) <= 0) break;
        if (m.message == WM_TIMER && m.hwnd == nullptr && m.wParam == timer) { timedOut = true; break; }
        TranslateMessage(&m);
        DispatchMessageW(&m);
    }
    KillTimer(nullptr, timer);

    int code;
    if (timedOut) {
        op.Cancel();
        code = Emit("error", "timeout", 4);
    } else if (op.Status() == AsyncStatus::Completed) {
        code = FromResult(op.GetResults());
    } else {
        code = Emit("error", "verification did not complete", 3);
    }
    if (created) DestroyWindow(created);
    return code;
}

}  // namespace

int wmain(int argc, wchar_t** argv) {
    init_apartment(apartment_type::multi_threaded);
    try {
        std::wstring cmd = argc > 1 ? argv[1] : L"";
        if (cmd == L"check" && argc == 2) return Check();
        if (cmd == L"verify") {
            std::wstring message = L"Multiplex needs your approval";
            unsigned seconds = 60;
            for (int i = 2; i < argc; i += 2) {
                if (i + 1 >= argc) return Emit("error", "missing value for " + ToUtf8(argv[i]), 3);
                std::wstring k = argv[i];
                if (k == L"--message") message = argv[i + 1];
                else if (k == L"--timeout") {
                    seconds = (unsigned)wcstoul(argv[i + 1], nullptr, 10);
                    if (seconds == 0 || seconds > 3600) return Emit("error", "--timeout must be 1..3600", 3);
                } else return Emit("error", "unknown argument " + ToUtf8(k), 3);
            }
            return Verify(message, seconds);
        }
        return Emit("error", "usage: mpx-hello check | verify --message <text> [--timeout <seconds>]", 3);
    } catch (hresult_error const& e) {
        return Emit("error", ToUtf8(e.message().c_str()), 3);
    } catch (...) {
        return Emit("error", "unexpected failure", 3);
    }
}
