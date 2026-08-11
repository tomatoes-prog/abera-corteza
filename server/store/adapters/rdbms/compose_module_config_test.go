package rdbms

import (
	"bytes"
	"testing"

	composeTypes "github.com/cortezaproject/corteza/server/compose/types"
	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/require"
)

func TestComposeModuleInsertQueryEncodesConfig(t *testing.T) {
	m := &composeTypes.Module{
		ID:          1,
		NamespaceID: 2,
		Handle:      "test",
		Name:        "Test",
		Config: composeTypes.ModuleConfig{
			DAL: composeTypes.ModuleConfigDAL{ConnectionID: 3},
		},
	}

	_, args, err := composeModuleInsertQuery(goqu.Dialect("sqlite3"), m).Prepared(true).ToSQL()
	require.NoError(t, err)

	var encoded []byte
	for _, arg := range args {
		if raw, ok := arg.([]byte); ok && bytes.Contains(raw, []byte(`"connectionID":"3"`)) {
			encoded = raw
		}
	}

	require.NotNil(t, encoded)
	require.JSONEq(t, `{"dal":{"connectionID":"3","constraints":null,"ident":"","systemFieldEncoding":{"id":null,"moduleID":null,"namespaceID":null,"revision":null,"meta":null,"ownedBy":null,"createdAt":null,"createdBy":null,"updatedAt":null,"updatedBy":null,"deletedAt":null,"deletedBy":null}},"privacy":{"usageDisclosure":""},"discovery":{"public":{"result":null},"private":{"result":null},"protected":{"result":null}},"recordRevisions":{"enabled":false,"ident":""},"recordDeDup":{}}`, string(encoded))
}
