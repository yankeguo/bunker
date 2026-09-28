package bunker

import (
	"sort"

	"github.com/git-lfs/wildmatch"
	"github.com/yankeguo/bunker/model"
)

type grantedItem struct {
	ServerUser string `json:"server_user"`
	ServerID   string `json:"server_id"`
}

func grantMatches(grant *model.Grant, serverUser, serverID string) bool {
	if grant == nil {
		return false
	}
	userMatch := wildmatch.NewWildmatch(grant.ServerUser, wildmatch.Basename, wildmatch.CaseFold)
	idMatch := wildmatch.NewWildmatch(grant.ServerID, wildmatch.Basename, wildmatch.CaseFold)
	return userMatch.Match(serverUser) && idMatch.Match(serverID)
}

// expandGrantedItems lists every server a user can reach, once per distinct
// server-user pattern. Patterns are not collapsed to a single row.
func expandGrantedItems(grants []*model.Grant, servers []*model.Server) []grantedItem {
	type grantMatcher struct {
		serverUser string
		matcher    *wildmatch.Wildmatch
	}

	matchers := make([]grantMatcher, 0, len(grants))
	for _, grant := range grants {
		if grant == nil {
			continue
		}
		matchers = append(matchers, grantMatcher{
			serverUser: grant.ServerUser,
			matcher: wildmatch.NewWildmatch(
				grant.ServerID,
				wildmatch.Basename,
				wildmatch.CaseFold,
			),
		})
	}

	items := []grantedItem{}
	for _, server := range servers {
		if server == nil {
			continue
		}

		seen := map[string]struct{}{}
		names := make([]string, 0, 1)
		for _, m := range matchers {
			if _, ok := seen[m.serverUser]; ok {
				continue
			}
			if m.matcher.Match(server.ID) {
				seen[m.serverUser] = struct{}{}
				names = append(names, m.serverUser)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			items = append(items, grantedItem{
				ServerUser: name,
				ServerID:   server.ID,
			})
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ServerID == items[j].ServerID {
			return items[i].ServerUser < items[j].ServerUser
		}
		return items[i].ServerID < items[j].ServerID
	})
	return items
}
