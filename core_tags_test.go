package jinja2

import (
  "testing"
)

func TestForLoopSimple(t *testing.T) {
  context := NewContext(map[string]interface{} {
      "seq": []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
    },
  )
  template := new(Template)
  err := template.Parse("{% for item in seq %}{{ item }}{% endfor %}")
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  expected := "0123456789"
  if res, err := template.Render(context); err != nil {
    t.Errorf("error rendering template:", err)
  } else if res != expected {
    t.Errorf("Template result was incorrect. Got: '%s' but expected '%s'", res, expected)
  }
}

func TestForLoopMultipleValues(t *testing.T) {
  context := NewContext(map[string]interface{} {
      "seq": []interface{}{
        []interface{}{0, 1},
        []interface{}{2, 3},
        []interface{}{4, 5},
      },
    },
  )
  template := new(Template)
  err := template.Parse("{% for a,b in seq %}{{a}}{{b}}{% endfor %}")
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  expected := "012345"
  if res, err := template.Render(context); err != nil {
    t.Errorf("error rendering template:", err)
  } else if res != expected {
    t.Errorf("Template result was incorrect. Got: '%s' but expected '%s'", res, expected)
  }
}

func TestForLoopElse(t *testing.T) {
  context := NewContext(map[string]interface{} {
      "seq": make([]interface{}, 0),
    },
  )
  template := new(Template)
  err := template.Parse("{% for item in seq %}XXX{% else %}...{% endfor %}")
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  expected := "..."
  if res, err := template.Render(context); err != nil {
    t.Errorf("error rendering template:", err)
  } else if res != expected {
    t.Errorf("Template result was incorrect. Got: '%s' but expected '%s'", res, expected)
  }
}
