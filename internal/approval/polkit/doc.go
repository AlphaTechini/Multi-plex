// Package polkit verifies a human on Linux desktops by asking polkit to
// authenticate the user through pkexec. A running polkit authentication agent
// (GNOME Shell, KDE, polkit-gnome, ...) shows the password or fingerprint
// dialog, so the agent CLI in the terminal cannot answer it.
//
// pkexec exit status (pkexec(1)): the program's own status on success; 126 if
// the user dismissed the authentication dialog; 127 if authorization could not
// be obtained (wrong password, not authorized, no agent, other errors).
//
// Limits: pkexec shows its own fixed text ("Authentication is needed to run
// /usr/bin/true as the super user"), not the command being approved; the
// Multiplex UI shows the real details. The check is only as strong as the
// polkit agent and the user's password; it proves a human at the desktop
// session, not which command they read.
package polkit
