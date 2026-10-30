package daemon

import "sort"

// sortSessions orders by creation time, then ID, so lists are stable.
func sortSessions(list []*session) {
	type key struct {
		s *session
		t int64
		i string
	}
	keys := make([]key, len(list))
	for i, s := range list {
		info := s.snap()
		keys[i] = key{s, info.CreatedAt.UnixNano(), string(info.ID)}
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].t != keys[b].t {
			return keys[a].t < keys[b].t
		}
		return keys[a].i < keys[b].i
	})
	for i := range keys {
		list[i] = keys[i].s
	}
}
