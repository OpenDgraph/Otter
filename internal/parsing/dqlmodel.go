package parsing

import (
	"github.com/hypermodeinc/dgraph/v24/protos/pb"

	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"
)

type Vars struct {
	Defines []string `json:"defines,omitempty"`
	Needs   []string `json:"needs,omitempty"`
}

type Result struct {
	Query     []*GraphQuery     `json:"query,omitempty"`
	QueryVars []*Vars           `json:"queryVars,omitempty"`
	Schema    *pb.SchemaRequest `json:"schema,omitempty"`
}

type GraphQuery struct {
	UID              []uint64                `json:"uid,omitempty"`
	Attr             string                  `json:"attr,omitempty"`
	Langs            []string                `json:"langs,omitempty"`
	Alias            string                  `json:"alias,omitempty"`
	IsCount          bool                    `json:"isCount,omitempty"`
	IsInternal       bool                    `json:"isInternal,omitempty"`
	IsGroupby        bool                    `json:"isGroupby,omitempty"`
	Var              string                  `json:"var,omitempty"`
	NeedsVar         []dqlpkg.VarContext     `json:"needsVar,omitempty"`
	Func             *dqlpkg.Function        `json:"func,omitempty"`
	Expand           string                  `json:"expand,omitempty"`
	Args             map[string]string       `json:"args,omitempty"`
	Order            []*pb.Order             `json:"order,omitempty"`
	Children         []*GraphQuery           `json:"children,omitempty"`
	Filter           *dqlpkg.FilterTree      `json:"filter,omitempty"`
	MathExp          *dqlpkg.MathTree        `json:"mathExp,omitempty"`
	Normalize        bool                    `json:"normalize,omitempty"`
	Recurse          bool                    `json:"recurse,omitempty"`
	RecurseArgs      dqlpkg.RecurseArgs      `json:"recurseArgs,omitempty"`
	ShortestPathArgs dqlpkg.ShortestPathArgs `json:"shortestPathArgs,omitempty"`
	Cascade          []string                `json:"cascade,omitempty"`
	IgnoreReflex     bool                    `json:"ignoreReflex,omitempty"`
	Facets           *pb.FacetParams         `json:"facets,omitempty"`
	FacetsFilter     *dqlpkg.FilterTree      `json:"facetsFilter,omitempty"`
	GroupbyAttrs     []dqlpkg.GroupByAttr    `json:"groupbyAttrs,omitempty"`
	FacetVar         map[string]string       `json:"facetVar,omitempty"`
	FacetsOrder      []*dqlpkg.FacetOrder    `json:"facetsOrder,omitempty"`
	AllowedPreds     []string                `json:"allowedPreds,omitempty"`
	IsEmpty          bool                    `json:"isEmpty,omitempty"`

	fragment string `json:"-"` // omitido sempre, interno
}
