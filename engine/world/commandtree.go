package world

import (
	"context"
	"sort"
	"strings"
)

// commandNode mirrors TrinityCore ChatCommandNode resolution: tokens are
// matched case-insensitively against children by unique prefix, aliases
// cover names that are not prefixes, and ambiguous tokens list candidates.
type commandNode struct {
	name     string
	children map[string]*commandNode
	aliases  map[string]string // alias -> canonical child name
	invoke   func(ctx context.Context, args []string) bool
}

func (n *commandNode) add(name string, invoke func(ctx context.Context, args []string) bool, subs []string, aliases map[string]string) *commandNode {
	child := &commandNode{name: name, invoke: invoke, children: make(map[string]*commandNode), aliases: make(map[string]string)}
	for _, sub := range subs {
		child.children[sub] = &commandNode{name: sub}
	}
	for alias, target := range aliases {
		child.aliases[alias] = target
	}
	n.children[name] = child
	return child
}

// commandTokens holds the canonicalized tokens once resolution completes.
type commandTokens []string

// resolve walks the tree token by token, rewriting each token to the
// canonical child name it matched (prefix or alias). Returns the deepest
// node and the rewritten tokens; consumed counts matched structural tokens.
func (n *commandNode) resolve(tokens []string) (*commandNode, commandTokens, int, []string, int) {
	node := n
	rewritten := make(commandTokens, 0, len(tokens))
	consumed := 0
	nodeDepth := 0
	// Once an invoke-bearing node is reached, remaining tokens are its
	// handler's parameters: they are still canonicalized against its
	// children (so 'spee' -> 'speed') but do not descend further.
	for _, token := range tokens {
		lower := strings.ToLower(token)
		matches := make([]string, 0, 2)
		for name := range node.children {
			if strings.HasPrefix(name, lower) {
				matches = append(matches, name)
			}
		}
		if target, ok := node.aliases[lower]; ok {
			duplicate := false
			for _, existing := range matches {
				if existing == target {
					duplicate = true
					break
				}
			}
			if !duplicate {
				matches = append(matches, target)
			}
		}
		if len(matches) == 0 {
			break
		}
		// Exact name matches take priority over partial ones
		// (ChatCommandNode::TryExecuteCommand skips the ambiguity check
		// when the token equals a child name verbatim).
		exact := make([]string, 0, 1)
		for _, candidate := range matches {
			if candidate == lower {
				exact = append(exact, candidate)
			}
		}
		if len(exact) == 1 {
			matches = exact
		}
		if len(matches) > 1 {
			sort.Strings(matches)
			return node, rewritten, consumed, matches, nodeDepth
		}
		canonical := matches[0]
		child := node.children[canonical]
		if child == nil {
			break
		}
		if node.invoke != nil {
			// Parameter canonicalization for the already-selected handler.
			rewritten = append(rewritten, canonical)
			consumed++
			continue
		}
		node = child
		nodeDepth = len(rewritten) + 1
		rewritten = append(rewritten, canonical)
		consumed++
	}
	return node, rewritten, consumed, nil, nodeDepth
}

// buildCommandTree assembles the command hierarchy with the canonical
// TrinityCore names; aliases mark non-prefix spellings.
func (s *session) buildCommandTree() *commandNode {
	root := &commandNode{name: "", children: make(map[string]*commandNode)}
	root.add("help", func(ctx context.Context, args []string) bool { s.handleCmdHelp(args); return true }, nil, map[string]string{"?": "help"})
	root.add("gm", func(ctx context.Context, args []string) bool { s.handleCmdGM(args); return true }, []string{"on", "off", "chat", "fly", "visible"}, map[string]string{"vis": "visible"})
	root.add("tele", func(ctx context.Context, args []string) bool { s.handleCmdTele(ctx, args); return true }, nil, nil)
	root.add("go", func(ctx context.Context, args []string) bool { s.handleCmdGo(ctx, args); return true }, nil, nil)
	root.add("modify", func(ctx context.Context, args []string) bool { s.handleCmdModify(ctx, args); return true }, []string{"hp", "health", "mana", "power", "speed", "run", "fly", "scale", "money", "gold", "level"}, map[string]string{"mod": "modify"})
	root.add("additem", func(ctx context.Context, args []string) bool { s.handleCmdAddItem(ctx, args); return true }, nil, map[string]string{"item": "additem"})
	root.add("learn", func(ctx context.Context, args []string) bool { s.handleCmdLearn(ctx, args); return true }, nil, nil)
	root.add("unlearn", func(ctx context.Context, args []string) bool { s.handleCmdUnlearn(ctx, args); return true }, nil, nil)
	root.add("cast", func(ctx context.Context, args []string) bool { s.handleCmdCast(ctx, args); return true }, nil, nil)
	root.add("lookup", func(ctx context.Context, args []string) bool { s.handleCmdLookup(ctx, args); return true }, []string{"item", "spell", "creature", "npc", "tele", "quest"}, nil)
	root.add("server", func(ctx context.Context, args []string) bool { s.handleCmdServer(ctx, args); return true }, []string{"info", "motd", "restart", "shutdown"}, nil)
	root.add("character", func(ctx context.Context, args []string) bool { s.handleCmdCharacter(ctx, args); return true }, []string{"level", "rename", "customize", "changefaction", "changerace"}, map[string]string{"char": "character"})
	root.add("account", func(ctx context.Context, args []string) bool { s.handleCmdAccount(ctx, args); return true }, []string{"set", "password"}, map[string]string{"acct": "account"})
	root.add("npc", func(ctx context.Context, args []string) bool { s.handleCmdNPC(ctx, args); return true }, []string{"info", "say", "yell"}, nil)
	root.add("gobject", func(ctx context.Context, args []string) bool { s.handleCmdGObject(ctx, args); return true }, nil, map[string]string{"gob": "gobject"})
	root.add("revive", func(ctx context.Context, args []string) bool { s.handleCmdRevive(ctx, args); return true }, nil, map[string]string{"res": "revive"})
	root.add("dismount", func(ctx context.Context, args []string) bool { s.handleCmdDismount(ctx); return true }, nil, nil)
	root.add("save", func(ctx context.Context, args []string) bool { s.handleCmdSave(ctx); return true }, nil, map[string]string{"saveall": "save"})
	return root
}

// dispatchCommand resolves a partial command like '.mod spee 10' to its
// canonical invocation ('.modify speed 10') like ChatCommandNode::
// TryExecuteCommand, reporting ambiguous matches with the TC wording.
func (s *session) dispatchCommand(ctx context.Context, fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	root := s.buildCommandTree()
	node, rewritten, _, ambiguous, nodeDepth := root.resolve(fields)
	if ambiguous != nil {
		s.sendSysMessage("There are multiple commands matching '" + strings.ToLower(fields[len(rewritten)]) + "'. Did you mean:")
		for _, candidate := range ambiguous {
			s.sendSysMessage(candidate)
		}
		return true
	}
	if node == root {
		return false
	}
	args := make([]string, 0, len(fields)-nodeDepth)
	args = append(args, rewritten[nodeDepth:]...)
	args = append(args, fields[len(rewritten):]...)
	if node.invoke == nil {
		s.sendSysMessage("Usage: ." + strings.Join(rewritten, " "))
		return true
	}
	return node.invoke(ctx, args)
}
