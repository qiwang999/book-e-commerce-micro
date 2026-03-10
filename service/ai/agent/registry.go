package agent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
)

type ToolGroup string

const (
	GroupDiscovery   ToolGroup = "discovery"
	GroupShopping    ToolGroup = "shopping"
	GroupOrder       ToolGroup = "order"
	GroupOrderManage ToolGroup = "order_manage"
)

// GroupMeta holds all metadata for a ToolGroup. This is registered once via
// RegisterGroup and consumed by both the Librarian prompt builder and the
// IntentRouter — no hardcoded prompt fragments or keyword lists elsewhere.
type GroupMeta struct {
	Title      string   // Prompt section title, e.g. "搜索与发现"
	Footer     string   // Optional footer appended after tool list in prompt
	IntentHint string   // Short hint for LLM intent classification, e.g. "搜索、推荐、找书"
	Keywords   []string // Keywords for rule-based intent matching
}

type ToolEntry struct {
	Tool        tool.BaseTool
	Group       ToolGroup
	Name        string
	Description string
	Sensitive   bool
}

type ToolRegistry struct {
	entries    []ToolEntry
	byGroup   map[ToolGroup][]ToolEntry
	byName    map[string]ToolEntry
	groupMeta map[ToolGroup]*GroupMeta
	groupOrder []ToolGroup
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		byGroup:   make(map[ToolGroup][]ToolEntry),
		byName:    make(map[string]ToolEntry),
		groupMeta: make(map[ToolGroup]*GroupMeta),
	}
}

// RegisterGroup registers metadata for a ToolGroup. Should be called before
// registering tools that belong to the group.
func (r *ToolRegistry) RegisterGroup(group ToolGroup, meta GroupMeta) {
	r.groupMeta[group] = &meta
	r.groupOrder = append(r.groupOrder, group)
}

func (r *ToolRegistry) Register(entry ToolEntry) {
	r.entries = append(r.entries, entry)
	r.byGroup[entry.Group] = append(r.byGroup[entry.Group], entry)
	r.byName[entry.Name] = entry
}

func (r *ToolRegistry) GetByGroups(groups ...ToolGroup) []tool.BaseTool {
	seen := make(map[string]bool)
	var tools []tool.BaseTool
	for _, g := range groups {
		for _, e := range r.byGroup[g] {
			if !seen[e.Name] {
				seen[e.Name] = true
				tools = append(tools, e.Tool)
			}
		}
	}
	return tools
}

// GetPromptSection builds a "## 工具使用" section containing only the tools
// belonging to the requested groups. Each group becomes a "### title" block.
func (r *ToolRegistry) GetPromptSection(groups ...ToolGroup) string {
	var b strings.Builder
	b.WriteString("## 工具使用\n\n")
	for _, g := range groups {
		entries := r.byGroup[g]
		if len(entries) == 0 {
			continue
		}
		meta := r.groupMeta[g]
		title := string(g)
		if meta != nil && meta.Title != "" {
			title = meta.Title
		}
		b.WriteString(fmt.Sprintf("### %s\n", title))
		for _, e := range entries {
			prefix := ""
			if e.Sensitive {
				prefix = "⚠️ "
			}
			b.WriteString(fmt.Sprintf("- **%s**: %s%s\n", e.Name, prefix, e.Description))
		}
		if meta != nil && meta.Footer != "" {
			b.WriteString(meta.Footer + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Groups returns all registered group names in registration order.
func (r *ToolRegistry) Groups() []ToolGroup {
	return r.groupOrder
}

// GetGroupMeta returns the metadata for a group, or nil if not registered.
func (r *ToolRegistry) GetGroupMeta(group ToolGroup) *GroupMeta {
	return r.groupMeta[group]
}

func (r *ToolRegistry) AllTools() []tool.BaseTool {
	tools := make([]tool.BaseTool, 0, len(r.entries))
	for _, e := range r.entries {
		tools = append(tools, e.Tool)
	}
	return tools
}

func (r *ToolRegistry) Count() int {
	return len(r.entries)
}
