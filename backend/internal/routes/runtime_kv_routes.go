package routes

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/runtimeplane"
)

type runtimeParameterReader interface {
	GetParameter(projectID, environmentID uuid.UUID, key string) (runtimeplane.RuntimeParameter, bool)
	ListParameters(projectID, environmentID uuid.UUID, prefix string) []runtimeplane.RuntimeParameter
}

type consulKVPair struct {
	CreateIndex uint64 `json:"CreateIndex"`
	ModifyIndex uint64 `json:"ModifyIndex"`
	LockIndex   uint64 `json:"LockIndex"`
	Key         string `json:"Key"`
	Flags       uint64 `json:"Flags"`
	Value       string `json:"Value"`
	Session     string `json:"Session"`
}

// RegisterRuntimeKVRoutes implements the read-only subset of Consul's KV API
// that is useful for workloads. Mutation belongs exclusively to the Control
// Plane and is deliberately not exposed here.
func RegisterRuntimeKVRoutes(rg *gin.RouterGroup, resolver middleware.ProjectKeyResolver, params runtimeParameterReader) {
	route := rg.Group("/v1/kv")
	route.Use(middleware.RequireRuntimeAPIKey(resolver))
	route.GET("/*key", func(c *gin.Context) {
		projectID, environmentID := middleware.RuntimeScope(c)
		key := strings.TrimPrefix(c.Param("key"), "/")
		recurse := queryTrue(c, "recurse") || queryTrue(c, "keys")
		keysOnly := queryTrue(c, "keys")

		if !recurse {
			param, found := params.GetParameter(projectID, environmentID, key)
			if !found {
				c.Status(http.StatusNotFound)
				return
			}
			setConsulHeaders(c, uint64(param.Version))
			if queryTrue(c, "raw") {
				c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(param.Value))
				return
			}
			c.JSON(http.StatusOK, []consulKVPair{toConsulKVPair(param)})
			return
		}

		items := params.ListParameters(projectID, environmentID, key)
		if len(items) == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		index := uint64(0)
		for _, item := range items {
			if uint64(item.Version) > index {
				index = uint64(item.Version)
			}
		}
		setConsulHeaders(c, index)
		if keysOnly {
			c.JSON(http.StatusOK, consulKeys(items, key, c.Query("separator")))
			return
		}
		out := make([]consulKVPair, 0, len(items))
		for _, item := range items {
			out = append(out, toConsulKVPair(item))
		}
		c.JSON(http.StatusOK, out)
	})
}

func queryTrue(c *gin.Context, name string) bool {
	value, present := c.GetQuery(name)
	return present && value != "false" && value != "0"
}

func setConsulHeaders(c *gin.Context, index uint64) {
	c.Header("X-Consul-Index", strconv.FormatUint(index, 10))
	c.Header("X-Consul-Knownleader", "true")
	c.Header("X-Consul-Lastcontact", "0")
}

func toConsulKVPair(param runtimeplane.RuntimeParameter) consulKVPair {
	index := uint64(param.Version)
	return consulKVPair{
		CreateIndex: 1, ModifyIndex: index, LockIndex: 0, Key: param.Key,
		Flags: 0, Value: base64.StdEncoding.EncodeToString([]byte(param.Value)), Session: "",
	}
}

func consulKeys(items []runtimeplane.RuntimeParameter, prefix, separator string) []string {
	seen := make(map[string]struct{})
	for _, item := range items {
		key := item.Key
		if separator != "" {
			remainder := strings.TrimPrefix(key, prefix)
			if i := strings.Index(remainder, separator); i >= 0 {
				key = prefix + remainder[:i+len(separator)]
			}
		}
		seen[key] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
