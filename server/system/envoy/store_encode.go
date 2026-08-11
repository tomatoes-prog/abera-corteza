package envoy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cortezaproject/corteza/server/pkg/envoyx"
	"github.com/cortezaproject/corteza/server/pkg/rbac"
	"github.com/cortezaproject/corteza/server/store"
	"github.com/cortezaproject/corteza/server/system/types"
)

// syncUserRoleMembership applies the role references declared by a user
// resource. The generated store encoder persists the user itself, but role
// memberships live in the role_members table and are not fields on types.User.
// Without this step, users declared with `roles:` in Envoy YAML are created
// without their memberships.
func (e StoreEncoder) syncUserRoleMembership(ctx context.Context, s store.Storer, n *envoyx.Node, tree envoyx.Traverser) error {
	roleIDs := make(map[uint64]struct{})
	for fieldLabel, ref := range n.References {
		if !strings.HasPrefix(fieldLabel, "Roles.") {
			continue
		}

		roleID := safeParentID(tree, n, ref)
		if roleID == 0 {
			return fmt.Errorf("failed to resolve user role reference %s", fieldLabel)
		}
		roleIDs[roleID] = struct{}{}
	}

	// A user without a roles reference is intentionally left untouched. This
	// preserves existing memberships for legacy exports that do not include
	// role information. An explicit roles list always produces at least one
	// Roles.N reference.
	if len(roleIDs) == 0 {
		return nil
	}

	resource := fmt.Sprintf("corteza::system:user/%d", n.Resource.(*types.User).ID)
	members, _, err := store.SearchRoleMembers(ctx, s, types.RoleMemberFilter{Resource: resource})
	if err != nil {
		return err
	}
	for _, member := range members {
		if err = store.DeleteRoleMember(ctx, s, member); err != nil {
			return err
		}
	}
	for roleID := range roleIDs {
		if err = store.CreateRoleMember(ctx, s, &types.RoleMember{Resource: resource, RoleID: roleID}); err != nil {
			return err
		}
	}

	return nil
}

func (e StoreEncoder) prepare(ctx context.Context, p envoyx.EncodeParams, s store.Storer, rt string, nn envoyx.NodeSet) (err error) {
	switch rt {
	case rbac.RuleResourceType:
		return e.prepareRbacRule(ctx, p, s, nn)
	case types.ResourceTranslationResourceType:
		return e.prepareResourceTranslation(ctx, p, s, nn)
	case types.SettingValueResourceType:
		return e.prepareSetting(ctx, p, s, nn)
	}

	return
}

func (e StoreEncoder) encode(ctx context.Context, p envoyx.EncodeParams, s store.Storer, rt string, nn envoyx.NodeSet, tree envoyx.Traverser) (err error) {
	switch rt {
	case rbac.RuleResourceType:
		return e.encodeRbacRules(ctx, p, s, nn, tree)
	case types.ResourceTranslationResourceType:
		return e.encodeResourceTranslations(ctx, p, s, nn, tree)
	case types.SettingValueResourceType:
		return e.encodeSettings(ctx, p, s, nn, tree)
	}

	return
}

func (e StoreEncoder) setApplicationDefaults(res *types.Application) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	if res.Unify == nil {
		res.Unify = &types.ApplicationUnify{}
	}

	return
}

func (e StoreEncoder) validateApplication(res *types.Application) (err error) {
	return
}

func (e StoreEncoder) setApigwRouteDefaults(res *types.ApigwRoute) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateApigwRoute(res *types.ApigwRoute) (err error) {
	return
}

func (e StoreEncoder) setApigwFilterDefaults(res *types.ApigwFilter) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateApigwFilter(res *types.ApigwFilter) (err error) {
	return
}

func (e StoreEncoder) setAuthClientDefaults(res *types.AuthClient) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateAuthClient(res *types.AuthClient) (err error) {
	return
}

func (e StoreEncoder) setQueueDefaults(res *types.Queue) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateQueue(res *types.Queue) (err error) {
	return
}

func (e StoreEncoder) setReportDefaults(res *types.Report) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateReport(res *types.Report) (err error) {
	return
}

func (e StoreEncoder) setRoleDefaults(res *types.Role) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateRole(res *types.Role) (err error) {
	return
}

func (e StoreEncoder) setTemplateDefaults(res *types.Template) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateTemplate(res *types.Template) (err error) {
	return
}

func (e StoreEncoder) setUserDefaults(res *types.User) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateUser(res *types.User) (err error) {
	return
}

func (e StoreEncoder) setDalConnectionDefaults(res *types.DalConnection) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateDalConnection(res *types.DalConnection) (err error) {
	return
}

func (e StoreEncoder) setDalSensitivityLevelDefaults(res *types.DalSensitivityLevel) (err error) {
	if res.CreatedAt.IsZero() {
		res.CreatedAt = time.Now()
	}

	return
}

func (e StoreEncoder) validateDalSensitivityLevel(res *types.DalSensitivityLevel) (err error) {
	return
}
