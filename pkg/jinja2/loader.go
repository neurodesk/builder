package jinja2

type MemoryLoader map[string]string

func (m MemoryLoader) Load(name string) (string, error) {
    if s, ok := m[name]; ok {
        return s, nil
    }
    return "", ErrTemplateNotFound{name}
}

type ErrTemplateNotFound struct{ Name string }

func (e ErrTemplateNotFound) Error() string { return "template not found: " + e.Name }

