package test

import (
	"testing"

	"github.com/ultipa/ultipa-go-driver/v5/sdk/utils"
)

func TestIsGlobalUql2(t *testing.T) {
	tests := []struct {
		uql      string
		expected bool
	}{
		{`top().task("567")`, true},
		{`grant().node_privilege(["READ"]).on("",@, *).user("lzq")`, true},
		{`grant().edge_privilege(["READ"]).on("",@, *).user("lzq")`, true},
		{`grant().privilege(["READ"]).on("",@, *).user("lzq")`, true},
		{`grant().system().privilege(["STAT"]).user("lzq")`, true},
		{`show().graph()`, true},
	}

	for _, tt := range tests {
		t.Run(tt.uql, func(t *testing.T) {
			uqlItem := utils.NewUql(tt.uql)
			isGlobal := uqlItem.IsGlobal()
			if isGlobal != tt.expected {
				t.Errorf("%s is global: %v, expected: %v", tt.uql, isGlobal, tt.expected)
			}
		})
	}
}
