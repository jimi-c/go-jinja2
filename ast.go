package jinja2

import (
  "errors"
  "strings"
)

//-------------------------------------------------------------------------------------------------
// The PyCallable describes:
// 1) A method used to run something natively in go
// 2) A mapping of python variable types. If set to a
//    value other than PY_TYPE_UNDEFINED, this will
//    become the default value when the call is made.
type CallableArg struct {
  Name string
  Value VariableType
}
type PyCallable struct {
  Method func([]VariableType) (VariableType, error)
  Args []CallableArg
}

func CreateArgumentList(a *ArgList, c *Context) ([]CallableArg, error) {
  arg_list := make([]CallableArg, 0)
  if a != nil {
    arg_res, arg_err := a.Eval(c)
    if arg_err != nil {
      return nil, arg_err
    }
    if args, ok := arg_res.Data.([]CallableArg); !ok {
      return nil, errors.New("Could not convert arguments to a callable list for the test.")
    } else {
      arg_list = append(arg_list, args...)
    }
  }
  return arg_list, nil
}
func MakeCall(call PyCallable, incoming_args []CallableArg, c *Context) (VariableType, error) {
  // FIXME: deal with *args and **kwargs
  args := make([]VariableType, len(call.Args))
  set_list := make([]bool, len(call.Args))
  for idx, _ := range set_list {
    set_list[idx] = false
  }

  next_pos := 0
  doing_named_args := false
  if incoming_args != nil {
    for _, arg := range incoming_args {
      if arg.Name != "" {
        doing_named_args = true
        found := false
        for idx, call_arg := range call.Args {
          if arg.Name == call_arg.Name {
            set_list[idx] = true
            args[idx] = arg.Value
            found = true
            break
          }
        }
        if !found {
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("Unknown named arg:'" + arg.Name + "'")
        }
      } else {
        if doing_named_args {
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("Positional arg found after a named arg.")
        }
        set_list[next_pos] = true
        args[next_pos] = arg.Value
        next_pos += 1
      }
    }
  }
  // now we validate all args were set, and if not we use the
  // default value provided in the call args. If there is no
  // default specified, we return an error.
  for idx, set_status := range set_list {
    if !set_status {
      call_arg := call.Args[idx]
      if call_arg.Value.Type == PY_TYPE_UNDEFINED {
        return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("Required argument for call was not set.")
      } else {
        args[idx] = call_arg.Value
      }
    }
  }
  // Args are matched and validated, so we make the call
  return call.Method(args)
}
//-------------------------------------------------------------------------------------------------

func ProcessJ2Filters(val VariableType, filters []*J2Filter, c *Context) (VariableType, error) {
  running_res := val
  for _, filter := range filters {
    filter_name := *filter.Name
    if filter_func, ok := c.Filters[filter_name]; !ok {
      return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("the filter '" + filter_name + "' was not found.")
    } else {
      arg_list, arg_err := CreateArgumentList(filter.Args, c)
      if arg_err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, arg_err
      }
      // for filters and tests, the first argument to the call is
      // the current value to the left of the filter chain
      arg_list = append([]CallableArg{CallableArg{"val", val}}, arg_list...)
      new_res, err := MakeCall(filter_func, arg_list, c)
      if err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      }
      running_res = new_res
    }
  }
  return running_res, nil
}
//-------------------------------------------------------------------------------------------------
func ProcessJ2Test(val VariableType, test *J2Test, c *Context) (VariableType, error) {
  test_name := *test.Name
  if test_func, ok := c.Tests[test_name]; !ok {
    return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("the test '" + test_name + "' was not found.")
  } else {
    arg_list, arg_err := CreateArgumentList(test.Args, c)
    if arg_err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, arg_err
    }
    // for filters and tests, the first argument to the call is
    // the current value to the left of the filter chain
    arg_list = append([]CallableArg{CallableArg{"val", val}}, arg_list...)
    new_res, err := MakeCall(test_func, arg_list, c)
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, err
    }
    if *test.Negated == "not" {
      b_val, err := new_res.AsBool()
      if err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      }
      return VariableType{PY_TYPE_BOOL, !b_val}, nil
    }
    // FIXME: should all tests be bools?
    return new_res, nil
  }
}
//-------------------------------------------------------------------------------------------------
type VariableStatement struct {
  Test *Test `@@`
}
func (self *VariableStatement) Eval(c *Context) (VariableType, error) {
  return self.Test.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type ForStatement struct {
  TargetList *TargetList `"for" @@ `
  TestList   *TestList   `"in" @@`
  IfStatement *IfStatement `[ @@ ]`
  Recursive bool `[@"recursive"]`
}
//-------------------------------------------------------------------------------------------------
type IfStatement struct {
  Test *Test `"if" @@`
}
func (self *IfStatement) Eval(c *Context) (VariableType, error) {
  res, err := self.Test.Eval(c)
  if err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  }
  if v, err := res.AsBool(); err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  } else {
    return VariableType{PY_TYPE_BOOL, v}, nil
  }
}
//-------------------------------------------------------------------------------------------------
type ElifStatement struct {
  Test *Test `"elif" @@`
}
func (self *ElifStatement) Eval(c *Context) (VariableType, error) {
  res, err := self.Test.Eval(c)
  if err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  }
  if v, err := res.AsBool(); err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  } else {
    return VariableType{PY_TYPE_BOOL, v}, nil
  }
}
//-------------------------------------------------------------------------------------------------
type TestList struct {
  Tests []*Test `@@ { "," @@ }[","]`
}
//-------------------------------------------------------------------------------------------------
type Test struct {
  Or *OrTest ` @@ `
}
func (self *Test) Eval(c *Context) (VariableType, error) {
  return self.Or.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type OrTest struct {
  Ands []*AndTest `@@ { "or" @@ }`
}
func (self *OrTest) Eval(c *Context) (VariableType, error) {
  if self.Ands != nil {
    var cur_res VariableType
    for idx, and := range self.Ands {
      if and_res, err := and.Eval(c); err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      } else {
        if idx == 0 {
          cur_res = and_res
        } else {
          res_bool, err := cur_res.AsBool()
          if err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, err
          }
          and_bool, err := and_res.AsBool()
          if err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, err
          }
          or_result := res_bool || and_bool
          cur_res = VariableType{PY_TYPE_BOOL, or_result}
          // if any of the ors are true, the result will be
          // true, so short-circuit the first time we find one
          if or_result {
            break
          }
        }
      }
    }
    return cur_res, nil
  } else {
    return VariableType{PY_TYPE_BOOL, false}, nil
  }
}
//-------------------------------------------------------------------------------------------------
type AndTest struct {
  Nots []*NotTest `@@ { "and" @@ }`
}
func (self *AndTest) Eval(c *Context) (VariableType, error) {
  if self.Nots != nil {
    var cur_res VariableType
    for idx, not := range self.Nots {
      if not_res, err := not.Eval(c); err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      } else {
        if idx == 0 {
          cur_res = not_res
        } else {
          res_bool, err := cur_res.AsBool()
          if err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, err
          }
          not_bool, err := not_res.AsBool()
          if err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, err
          }
          and_result := res_bool && not_bool
          cur_res = VariableType{PY_TYPE_BOOL, and_result}
          // if any of the ands are false, the result will be
          // false, so short-circuit the first time we find one
          if !and_result {
            break
          }
        }
      }
    }
    return cur_res, nil
  } else {
    return VariableType{PY_TYPE_BOOL, false}, nil
  }
}
//-------------------------------------------------------------------------------------------------
type NotTest struct {
  Negated    *NotTest    `"not" @@`
  Comparison *Comparison `| @@`
}
func (self *NotTest) Eval(c *Context) (VariableType, error) {
  if self.Negated != nil {
    res, err := self.Negated.Eval(c)
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, err
    } else {
      if v, ok := res.Data.(bool); !ok {
        return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("error converting 'not' result to a boolean value for negation")
      } else {
        return VariableType{PY_TYPE_BOOL, !v}, nil
      }
    }
  } else if self.Comparison != nil {
    return self.Comparison.Eval(c)
  } else {
    return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("no negated expression nor a comparison was found")
  }
}
//-------------------------------------------------------------------------------------------------
type Comparison struct {
  Expr *Expr `@@`
  OpExpr []*OpExpr `{ @@ }`
}
func (self *Comparison) Eval(c *Context) (VariableType, error) {
  if self.Expr == nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("expression in comparison is nil")
  }
  l_res, l_err := self.Expr.Eval(c)
  if l_err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, l_err
  }
  defer func() (VariableType, error) {
    return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unable to compare data")
  }()
  if self.OpExpr != nil {
    final_result := true
    for _, opexpr := range self.OpExpr {
      r_res, r_err := opexpr.Eval(c)
      if r_err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, r_err
      }
      if l_res.Type != r_res.Type {
        return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("mismatched types for comparison")
      }
      switch *opexpr.Op {
      case "<":
        switch l_res.Type {
        case PY_TYPE_INT:
          final_result = final_result && l_res.Data.(int64) < r_res.Data.(int64)
        default:
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("invalid comparison op'<' for type '" + PyTypeToString(l_res.Type) + "'")
        }
      case ">":
        switch l_res.Type {
        case PY_TYPE_INT:
          l_val := l_res.Data.(int64)
          r_val := r_res.Data.(int64)
          final_result = final_result && l_val > r_val
        default:
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("invalid comparison op'>' for type '" + PyTypeToString(l_res.Type) + "'")
        }
      case "<=":
        switch l_res.Type {
        case PY_TYPE_INT:
          final_result = final_result && l_res.Data.(int64) <= r_res.Data.(int64)
        default:
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("invalid comparison op'<=' for type '" + PyTypeToString(l_res.Type) + "'")
        }
      case ">=":
        switch l_res.Type {
        case PY_TYPE_INT:
          final_result = final_result && l_res.Data.(int64) >= r_res.Data.(int64)
        default:
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("invalid comparison op'>=' for type '" + PyTypeToString(l_res.Type) + "'")
        }
      case "==":
        switch l_res.Type {
        case PY_TYPE_INT:
          final_result = final_result && l_res.Data.(int64) == r_res.Data.(int64)
        default:
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("invalid comparison op'==' for type '" + PyTypeToString(l_res.Type) + "'")
        }
      case "!=":
        switch l_res.Type {
        case PY_TYPE_INT:
          final_result = final_result && l_res.Data.(int64) != r_res.Data.(int64)
        default:
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("invalid comparison op'==' for type '" + PyTypeToString(l_res.Type) + "'")
        }
      }
      l_res = r_res
    }
    return VariableType{PY_TYPE_BOOL, final_result}, nil
  } else {
    return l_res, nil
  }
}
//-------------------------------------------------------------------------------------------------
type Expr struct {
  Lhs  *ArithExpr     `@@`
  Rhs  []*OpArithExpr `{ @@ }`
}
func (self *Expr) Eval(c *Context) (VariableType, error) {
  l_res, err := self.Lhs.Eval(c)
  if err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  }
  if self.Rhs != nil {
    cur_res := l_res
    for _, rhs := range self.Rhs {
      if rhs_res, err := rhs.Eval(c); err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      } else {
        if cur_res.Type == PY_TYPE_FLOAT || rhs_res.Type == PY_TYPE_FLOAT {
          // need to convert to floats
          l_val, l_err := cur_res.AsFloat()
          r_val, r_err := rhs_res.AsFloat()
          if l_err != nil || r_err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
          }
          switch *rhs.Op {
          case "+":
            cur_res = VariableType{PY_TYPE_FLOAT, l_val + r_val}
          case "-":
            cur_res = VariableType{PY_TYPE_FLOAT, l_val - r_val}
          }
        } else if cur_res.Type == PY_TYPE_INT || rhs_res.Type == PY_TYPE_INT {
          // convert both to integers
          l_val, l_err := cur_res.AsInt()
          r_val, r_err := rhs_res.AsInt()
          if l_err != nil || r_err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
          }
          switch *rhs.Op {
          case "+":
            cur_res = VariableType{PY_TYPE_INT, l_val + r_val}
          case "-":
            cur_res = VariableType{PY_TYPE_INT, l_val - r_val}
          }
        } else if cur_res.Type == PY_TYPE_STRING && rhs_res.Type == PY_TYPE_STRING {
          // string join
          if *rhs.Op == "-" {
            return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (string) and (string))")
          }
          l_val, l_err := cur_res.AsString()
          r_val, r_err := rhs_res.AsString()
          if l_err != nil || r_err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
          }
          cur_res = VariableType{PY_TYPE_STRING, l_val + r_val}
        } else {
          // error, can't do math between disparate types
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
        }
      }
    }
    return cur_res, nil
  } else {
    return l_res, nil
  }
}
//-------------------------------------------------------------------------------------------------
type OpExpr struct {
  Op  *string  `@("<"|">"|"=="|">="|"<="|"<>"|"!="|"in"|"is"|"not" "in"|"is" "not")`
  ArithExpr *ArithExpr `@@`
}
func (self *OpExpr) Eval(c *Context) (VariableType, error) {
  return self.ArithExpr.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type ArithExpr struct {
  Lhs *Term     `@@`
  Rhs []*OpTerm `{ @@ }`
}
func (self *ArithExpr) Eval(c *Context) (VariableType, error) {
  l_res, err := self.Lhs.Eval(c)
  if err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  }
  if self.Rhs != nil {
    cur_res := l_res
    for _, rhs := range self.Rhs {
      if rhs_res, err := rhs.Eval(c); err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      } else {
        if cur_res.Type == PY_TYPE_FLOAT || rhs_res.Type == PY_TYPE_FLOAT {
          // need to convert to floats
          l_val, l_err := cur_res.AsFloat()
          r_val, r_err := rhs_res.AsFloat()
          if l_err != nil || r_err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
          }
          switch *rhs.Op {
          case "*":
            cur_res = VariableType{PY_TYPE_FLOAT, l_val * r_val}
          case "/":
            cur_res = VariableType{PY_TYPE_FLOAT, l_val / r_val}
          }
        } else if cur_res.Type == PY_TYPE_INT || rhs_res.Type == PY_TYPE_INT {
          // convert both to integers
          l_val, l_err := cur_res.AsInt()
          r_val, r_err := rhs_res.AsInt()
          if l_err != nil || r_err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
          }
          switch *rhs.Op {
          case "*":
            cur_res = VariableType{PY_TYPE_INT, l_val * r_val}
          case "/":
            cur_res = VariableType{PY_TYPE_INT, l_val / r_val}
          }
        } else {
          // FIXME: allow `"string" * int` expr
          // error, can't do math between disparate types
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported op '"+ (*rhs.Op) +"' for a (fixme) and (fixme))")
        }
      }
    }
    return cur_res, nil
  } else {
    return l_res, nil
  }
}
//-------------------------------------------------------------------------------------------------
type OpArithExpr struct {
  Op *string `@("+"|"-")`
  Term *Term `@@`
}
func (self *OpArithExpr) Eval(c *Context) (VariableType, error) {
  return self.Term.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type Term struct {
  Factor  *Factor   `@@`
}
func (self *Term) Eval(c *Context) (VariableType, error) {
  return self.Factor.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type OpTerm struct {
  Op     *string `@("*"|"/"|"%"|"//")`
  Factor *Factor `@@`
}
func (self *OpTerm) Eval(c *Context) (VariableType, error) {
  return self.Factor.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type Factor struct {
  ModFactor *ModFactor `  @@`
  Power     *Power     `| @@`
}
func (self *Factor) Eval(c *Context) (VariableType, error) {
  if self.ModFactor != nil {
    return self.ModFactor.Eval(c)
  } else if self.Power != nil {
    return self.Power.Eval(c)
  } else {
    return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("Neither a modfactor nor a power were found while parsing factor.")
  }
}
//-------------------------------------------------------------------------------------------------
type ModFactor struct {
  Mod    *string `@("+"|"-"|"~")`
  Factor *Factor `@@`
}
func (self *ModFactor) Eval(c *Context) (VariableType, error) {
  res, err := self.Factor.Eval(c)
  if err != nil {
    return VariableType{PY_TYPE_UNDEFINED, nil}, err
  }
  switch res.Type {
  case PY_TYPE_INT, PY_TYPE_BOOL:
    i_val, err := res.AsInt()
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("error while converting variable to an integer")
    } else {
      switch *self.Mod {
      case "+":
        // this doesn't actually do anything in python...
      case "-":
        i_val = -i_val
      case "~":
        i_val = -(i_val+1)
      }
      return VariableType{PY_TYPE_INT, i_val}, nil
    }
  case PY_TYPE_FLOAT:
    f_val, err := res.AsFloat()
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("error while converting variable to an integer")
    } else {
      switch *self.Mod {
      case "+":
        // this doesn't actually do anything in python...
      case "-":
        f_val = -f_val
      case "~":
        return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported unary operation '~' on a float")
      }
      return VariableType{PY_TYPE_FLOAT, f_val}, nil
    }
  default:
    return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("unsupported unary operation '"+(*self.Mod)+"' on a FIXME")
  }
}
//-------------------------------------------------------------------------------------------------
type Power struct {
  AtomExpr *AtomExpr   `@@`
  Factor   *Factor     `[ "**" @@ ]`
  Filters  []*J2Filter `{ "|" @@ }`
  Test     *J2Test     `[ @@ ]`
}
func (self *Power) Eval(c *Context) (VariableType, error) {
  atom_res, err := self.AtomExpr.Eval(c)
  var deferred_err error = nil
  if err != nil {
    // FIXME: not sure if this is the best way to handle this, as
    //        the deferred error will most likely be ignored if any
    //        test is successfully run, masking the error
    if self.Test == nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, err
    } else {
      deferred_err = err
    }
  }
  /*
  // FIXME: implement powers
  if self.Factor != nil {
    r_atom_type, r_atom_res, r_err := self.Factor.Eval(c)
  }
  */
  return_res := atom_res
  if self.Filters != nil {
    res, err := ProcessJ2Filters(atom_res, self.Filters, c)
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, err
    }
    return_res = res
  }
  if self.Test != nil {
    res, err := ProcessJ2Test(return_res, self.Test, c)
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, err
    }
    deferred_err = nil
    return_res = res
  }
  return return_res, deferred_err
}
//-------------------------------------------------------------------------------------------------
type AtomExpr struct {
  Atom *Atom `@@`
  Trailers []*Trailer `{ @@ }`
}
func (self *AtomExpr) Eval(c *Context) (VariableType, error) {
  atom_res, err := self.Atom.Eval(c)
  if self.Trailers != nil {
    for _, t := range self.Trailers {
      if t.Name != nil {
        // this is a sub-key in a dictionary or an attribute on the
        // class, so we set the running value to whichever it is.
        if atom_res.Type == PY_TYPE_DICT {
          if sub_dict, ok := atom_res.Data.(map[VariableType]VariableType); !ok {
            // FIXME: error
          } else {
            if v, ok := sub_dict[VariableType{PY_TYPE_STRING, *t.Name}]; !ok {
              // FIXME: error
            } else {
              atom_res = v
            }
          }
        } else {
          // class/struct?
        }
      } else if t.ArgList != nil {
        // this is a callable, so we need to lookup which
        // method is being called and pass the args to it, then
        // we assign the result to the running value.
        if atom_res.Type != PY_TYPE_IDENT {
          // FIXME: better error here
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("Cannot make a call on a non-identifier.")
        }
        call_name := atom_res.Data.(string)
        if call_func, ok := c.PyCalls[call_name]; !ok {
          return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("the method '" + call_name + "' was not found.")
        } else {
          arg_list, arg_err := CreateArgumentList(t.ArgList, c)
          if arg_err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, arg_err
          }
          new_res, err := MakeCall(call_func, arg_list, c)
          if err != nil {
            return VariableType{PY_TYPE_UNDEFINED, nil}, err
          }
          atom_res = new_res
        }
      }
    }
  }
  // if we have an identifier, we do the variable lookup here
  if atom_res.Type == PY_TYPE_IDENT {
    var_name := atom_res.Data.(string)
    if v, ok := c.Variables[var_name]; ok {
      return v, nil
    } else {
      return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("variable name '"+var_name+"' was not found in the current context.")
    }
  } else {
    return atom_res, err
  }
}
//-------------------------------------------------------------------------------------------------
type Atom struct {
  Name      *string      `  @Ident`
  Str       *string      `| @String`
  Float     *float64     `| @Float`
  Int       *int64       `| @Int`
  Bool      *string      `| @Bool`
  None      *string      `| @None`
  List      *ListDisplay `| @@`
  Dict      *DictDisplay `| @@`
}
func (self *Atom) Eval(c *Context) (VariableType, error) {
  if self.Name != nil { return VariableType{PY_TYPE_IDENT, *self.Name}, nil
  } else if self.Str != nil { return VariableType{PY_TYPE_STRING, *self.Str}, nil
  } else if self.Float != nil { return VariableType{PY_TYPE_FLOAT, *self.Float}, nil
  } else if self.Int != nil { return VariableType{PY_TYPE_INT, *self.Int}, nil
  } else if self.Bool != nil {
    if strings.ToLower(*self.Bool) == "true" {
      return VariableType{PY_TYPE_BOOL, true}, nil
    } else {
      return VariableType{PY_TYPE_BOOL, false}, nil
    }
  } else if self.None != nil { return VariableType{PY_TYPE_NONE, nil}, nil
  } else if self.List != nil { return self.List.Eval(c)
  } else if self.Dict != nil { return self.Dict.Eval(c)
  } else { return VariableType{PY_TYPE_UNDEFINED, nil}, errors.New("atomic value was not set")
  }
}
//-------------------------------------------------------------------------------------------------
type Trailer struct {
  ArgList *ArgList `  @@`
  Name    *string  `| "." @Ident`
}
//-------------------------------------------------------------------------------------------------
type ArgList struct {
  Arguments []*Argument `"(" [ @@ { "," @@ }[","] ] ")"`
}
func (self *ArgList) Eval(c *Context) (VariableType, error) {
  args := make([]CallableArg, 0)
  if self.Arguments != nil {
    for _, arg := range(self.Arguments) {
      arg_val, err := arg.Eval(c)
      if err != nil {
        return VariableType{PY_TYPE_UNDEFINED, nil}, err
      }
      //if arg.NamedArg != nil {
      if arg.Name != nil {
        args = append(args, CallableArg{*arg.Name, arg_val})
      } else {
        args = append(args, CallableArg{"", arg_val})
      }
    }
  }
  return VariableType{PY_TYPE_LIST, args}, nil
}
//-------------------------------------------------------------------------------------------------
type Argument struct {
  //NamedArg  *NamedArgument  `  @@`
  //AnonArg   *AnonArgument   `| @@`
  AnonValue *Test `@@ |`
  Name *string `( @Ident`
  Value *Test  `"=" @@ )`
}
func (self *Argument) Eval(c *Context) (VariableType, error) {
  /*
  if self.NamedArg != nil {
    return self.NamedArg.Eval(c)
  } else {
    return self.AnonArg.Eval(c)
  }
  */
  return self.Value.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type NamedArgument struct {
  ArgName  *string ` ( @Ident "=" `
  ArgValue *Expr   ` @@ )`
}
func (self *NamedArgument) Eval(c *Context) (VariableType, error) {
  return self.ArgValue.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type AnonArgument struct {
  ArgValue *Expr `@@`
}
func (self *AnonArgument) Eval(c *Context) (VariableType, error) {
  return self.ArgValue.Eval(c)
}
//-------------------------------------------------------------------------------------------------
type ExprList struct {
  Exprs []*Expr `@@ {"," @@ }[","]`
}
//-------------------------------------------------------------------------------------------------
type ListDisplay struct {
  Items []*Expr `"[" [ @@ {"," @@ }[","] ] "]"`
}
func (self *ListDisplay) Eval(c *Context) (VariableType, error) {
  res := make([]VariableType, len(self.Items))
  for idx, item := range self.Items {
    item_res, err := item.Eval(c)
    if err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, err
    } else {
      res[idx] = item_res
    }
  }
  return VariableType{PY_TYPE_LIST, res}, nil
}
//-------------------------------------------------------------------------------------------------
type TupleDisplay struct {
  Items []*Expr `"(" @@ {"," @@ }[","] ")"`
}
//-------------------------------------------------------------------------------------------------
type DictDisplay struct {
	Entries []*KeyDatum `"{" [ @@ {"," @@ }[","] ] "}"`
}
func (self *DictDisplay) Eval(c *Context) (VariableType, error) {
  res := make(map[VariableType]VariableType)
  for _, item := range self.Entries {
    key_res, key_err := item.Key.Eval(c)
    val_res, val_err := item.Value.Eval(c)
    if key_err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, key_err
    } else if val_err != nil {
      return VariableType{PY_TYPE_UNDEFINED, nil}, val_err
    } else {
      res[key_res] = val_res
    }
  }
  return VariableType{PY_TYPE_DICT, res}, nil
}
//-------------------------------------------------------------------------------------------------
type KeyDatum struct {
	Key   *Expr `@@ ":"`
	Value *Expr `@@`
}
//-------------------------------------------------------------------------------------------------
type TargetList struct {
  Targets []*Target ` @@ { "," @@ }[","] `
}
//-------------------------------------------------------------------------------------------------
type Target struct {
  Name *string `@Ident`
}
//-------------------------------------------------------------------------------------------------
type J2Filter struct {
  Name *string  `@Ident`
  Args *ArgList `[ @@ ]`
}
type J2Test struct {
  Negated *string  `"is" @[ "not" ]`
  Name    *string  `@Ident`
  Args    *ArgList `[ @@ ]`
}
