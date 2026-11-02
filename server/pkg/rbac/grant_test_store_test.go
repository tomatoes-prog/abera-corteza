package rbac

import "context"

// nopRulesStore is the upstream test fixture needed by grant_flush_test.go.
type nopRulesStore struct{}

func (nopRulesStore) SearchRbacRules(context.Context, RuleFilter) (RuleSet, RuleFilter, error) {
	return nil, RuleFilter{}, nil
}
func (nopRulesStore) UpsertRbacRule(context.Context, ...*Rule) error { return nil }
func (nopRulesStore) DeleteRbacRule(context.Context, ...*Rule) error { return nil }
func (nopRulesStore) TruncateRbacRules(context.Context) error        { return nil }
