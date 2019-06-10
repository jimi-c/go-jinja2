package jinja2

import (
  "fmt"
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

func TestJinja2Filter(t *testing.T) {
  context := NewContext(map[string]interface{} {
    },
  )
  template := new(Template)
  err := template.Parse("{{'1'|int}}")
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  expected := "1"
  if res, err := template.Render(context); err != nil {
    t.Errorf("error rendering template:", err)
  } else if res != expected {
    t.Errorf("Template result was incorrect. Got: '%s' but expected '%s'", res, expected)
  }
}

func TestJinja2Test(t *testing.T) {
  context := NewContext(map[string]interface{} {
      "good_var": "yes",
    },
  )
  template := new(Template)
  err := template.Parse("{{good_var is defined}}")
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  expected := "true"
  if res, err := template.Render(context); err != nil {
    t.Errorf("error rendering template:", err)
  } else if res != expected {
    t.Errorf("Template result was incorrect. Got: '%s' but expected '%s'", res, expected)
  }
  err = template.Parse("{{missing_var is not defined}}")
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  expected = "true"
  if res, err := template.Render(context); err != nil {
    t.Errorf("error rendering template:", err)
  } else if res != expected {
    t.Errorf("Template result was incorrect. Got: '%s' but expected '%s'", res, expected)
  }
}

func TestCall(t *testing.T) {
  context := NewContext(map[string]interface{} {
      "foo": 1,
    },
  )
  context.PyCalls["test"] = PyCallable{
    func(args []VariableType) (VariableType, error) {
      a := args[0]
      b := args[1]
      c := args[2]
      return VariableType{PY_TYPE_LIST, []VariableType{
          VariableType{PY_TYPE_INT, a.Data},
          VariableType{PY_TYPE_STRING, b.Data},
          VariableType{PY_TYPE_BOOL, c.Data},
        },
      }, nil
    }, []CallableArg {
      {"a", VariableType{PY_TYPE_UNDEFINED, nil},},
      {"b", VariableType{PY_TYPE_UNDEFINED, nil},},
      {"c", VariableType{PY_TYPE_BOOL, true},},
    },
  }
  template := new(Template)
  err := template.Parse(`{{ test(foo, b="2") }}`)
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  fmt.Println(template.Render(context))
  err = template.Parse(`{{ test(1, "2", c=false) }}`)
  if err != nil {
    t.Errorf("error parsing template:", err)
  }
  fmt.Println(template.Render(context))
}
