package profilemodel

// TrieNode is shared by exact ProfileSet operations and the legacy integer view
// adapter. Identity is a caller-supplied key; display labels never merge frames.
type TrieNode[T any] struct {
	ID       string
	ParentID *string
	Name     string
	Value    T
	Children map[string]*TrieNode[T]
}
type Trie[T any] struct {
	Root  *TrieNode[T]
	add   func(T, T) T
	id    func(string, string) string
	zero  func() T
	Nodes int
}

func NewTrie[T any](rootID, name string, zero func() T, add func(T, T) T, id func(string, string) string) *Trie[T] {
	return &Trie[T]{Root: &TrieNode[T]{ID: rootID, Name: name, Value: zero(), Children: map[string]*TrieNode[T]{}}, zero: zero, add: add, id: id, Nodes: 1}
}
func (t *Trie[T]) Add(frames []string, value T) {
	current := t.Root
	current.Value = t.add(current.Value, value)
	for _, frame := range frames {
		child := current.Children[frame]
		if child == nil {

			child = &TrieNode[T]{Name: frame, Value: t.zero(), Children: map[string]*TrieNode[T]{}}
			if t.id != nil {
				parent := current.ID
				child.ID = t.id(parent, frame)
				child.ParentID = &parent
			}

			current.Children[frame] = child
			t.Nodes++
		}
		child.Value = t.add(child.Value, value)
		current = child
	}
}
