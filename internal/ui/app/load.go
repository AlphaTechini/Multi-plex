package app

import (
	"context"
	"errors"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// loadedMsg is the result of the initial data load.
type loadedMsg struct {
	ws        core.Workspace
	sessions  []core.Session
	specs     []protocol.SpecInfo
	agents    []protocol.AgentInfo
	roles     []protocol.RoleInfo
	approvals []core.ApprovalRequest
	notes     map[core.SessionID]int
	info      workspaceInfo
	err       error
}

// loadCmd opens the workspace, then loads sessions, specs, pending approvals,
// card counts and the on-disk workspace info in parallel.
func (m *Model) loadCmd() tea.Cmd {
	be, dir, probe := m.be, m.o.WorkspaceDir, m.probe
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ws, err := be.OpenWorkspace(ctx, dir)
		if err != nil {
			return loadedMsg{err: err}
		}
		out := loadedMsg{ws: ws, notes: map[core.SessionID]int{}}
		var wg sync.WaitGroup
		var mu sync.Mutex
		wg.Add(5)
		go func() {
			defer wg.Done()
			ss, err := be.Sessions(ctx, "")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				out.err = err
				return
			}
			out.sessions = ss
		}()
		go func() {
			defer wg.Done()
			sp, _ := be.Specs(ctx, ws.ID)
			mu.Lock()
			out.specs = sp
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			ag, _ := be.Agents(ctx)
			ro, _ := be.Roles(ctx, ws.ID)
			mu.Lock()
			out.agents, out.roles = ag, ro
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			ap, _ := be.PendingApprovals(ctx)
			mu.Lock()
			out.approvals = ap
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			info := probe(ws)
			mu.Lock()
			out.info = info
			mu.Unlock()
		}()
		wg.Wait()
		for _, s := range out.sessions {
			ns, err := be.Notes(ctx, s.ID)
			if err == nil {
				out.notes[s.ID] = countOpen(ns)
			}
		}
		return out
	}
}

func countOpen(ns []core.Note) int {
	n := 0
	for _, x := range ns {
		if !x.Dismissed {
			n++
		}
	}
	return n
}

// handleLoaded installs the loaded data.
func (m *Model) handleLoaded(l loadedMsg) tea.Cmd {
	var perr *protocol.Error
	if errors.As(l.err, &perr) && perr.Code == protocol.CodeNotARepo {
		m.loadErr, m.notRepo = perr.Message, true
		return nil
	}
	if l.err != nil {
		m.loadErr = l.err.Error()
		return m.addToast(toastSpec{tone: kit.ToneDanger, title: "Could not reach the daemon", body: l.err.Error(), sticky: true})
	}
	m.loaded = true
	m.ws = l.ws
	m.env.Workspace = l.ws
	m.specs, m.info = l.specs, l.info
	m.agents, m.roles = l.agents, l.roles
	if l.roles != nil {
		m.info.Roles = len(l.roles) // the daemon's list is the truth, not the files probe
	}
	m.notes = l.notes
	m.setSessions(l.sessions)
	var cmds []tea.Cmd
	for _, r := range l.approvals {
		m.approvals = append(m.approvals, pendingApproval{req: r})
	}
	if m.cur == "" && len(m.sessions) > 0 {
		cmds = append(cmds, m.selectSession(m.sessions[0].ID))
	} else {
		cmds = append(cmds, m.activateMode(m.mode))
	}
	m.sbClamp()
	return tea.Batch(cmds...)
}
