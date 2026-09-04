package envoy

import (
	"context"
	"fmt"
	"strings"

	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/cortezaproject/corteza/server/store"
	"github.com/cortezaproject/corteza/server/system/types"
	"github.com/spf13/cast"
)

type (
	// UserGetter is a utility struct to resolve user references from
	// different parts of the system such as the dep graph and the database
	UserGetter struct {
		depGraph envoyx.Traverser

		store      store.Storer
		baseFilter types.UserFilter

		// Cache resolved references, including misses, during one import.
		cache map[string]uint64
	}
)

func MakeUserGetter(s store.Storer, tt envoyx.Traverser) (g *UserGetter) {
	g = &UserGetter{
		depGraph: tt,
		store:    s,
		cache:    make(map[string]uint64),
	}

	g.baseFilter = types.UserFilter{}

	return
}

// Resolve returns the user ID for the provided reference
//
// If the user can not be resolved, 0 is returned.
func (g *UserGetter) Resolve(ctx context.Context, ref any) (out uint64, err error) {
	// Try to get from datasource
	if g.depGraph != nil {
		out, err = g.getDS(ref)
		if err != nil {
			return
		}
	}
	if out > 0 {
		return
	}

	// Try to get from the database
	out, err = g.getDB(ctx, ref)
	return
}

func (g *UserGetter) getDS(ref any) (out uint64, err error) {
	n := g.depGraph.NodeForRef(envoyx.Ref{
		ResourceType: types.UserResourceType,
		Identifiers:  envoyx.MakeIdentifiers(ref),
	})

	if n == nil {
		return
	}

	return n.Resource.GetID(), nil
}

// @todo this can be improved by prefetching and indexing refs
func (g *UserGetter) getDB(ctx context.Context, ref any) (out uint64, err error) {
	aux := strings.TrimSpace(cast.ToString(ref))

	// Nothing to resolve; without this the query below could match any user.
	if aux == "" {
		return 0, nil
	}

	// Numeric references are exported user IDs and must not be interpreted as
	// a name/email query that could match an unrelated user.
	if id := cast.ToUint64(aux); id > 0 {
		return id, nil
	}

	if out, ok := g.cache[aux]; ok {
		return out, nil
	}
	defer func() {
		if err == nil {
			g.cache[aux] = out
		}
	}()

	f := g.baseFilter
	f.Query = aux

	set, _, err := store.SearchUsers(ctx, g.store, f)
	if err != nil {
		return
	}

	if len(set) == 0 {
		return
	}

	if len(set) > 1 {
		err = fmt.Errorf("ambiguous reference %v: matches more then one user", ref)
		return
	}

	return set[0].ID, nil
}
