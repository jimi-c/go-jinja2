package jinja2

import (
  "errors"
  "strconv"
  "github.com/alecthomas/participle"
  "github.com/alecthomas/participle/lexer"
)

var (
	PythonLexer = lexer.Unquote(lexer.Must(lexer.Regexp(`(\s+)`+
    `|(?P<Bool>[Tt]rue|[Ff]alse)`+
    `|(?P<Ident>[a-zA-Z_][a-zA-Z0-9_]*)`+
    `|(?P<String>'[^']*'|"[^"]*")`+
    `|(?P<Operators>\||<>|==|!=|<=|>=|[-+*/%,.=<>])`+
    `|(?P<Delimiters>[()\[\]{}:])`+
    `|(?P<None>None)`+
    `|(?P<Float>[-+]?\d*\.\d+([eE][-+]?\d+)?)`+
    `|(?P<Int>[-+]?\d*)`+
    `|(?P<Keyword>or|and|is|in|not|if|elif|else)`,
	)), "String")
)

type Renderable interface {
  Render(*Context) (string, error)
}

type DummyChunk struct {
}
func (self *DummyChunk) Render(c *Context) (string, error) {
  return "", nil
}

type TextChunk struct {
  Text string
}
func (self *TextChunk) Render(c *Context) (string, error) {
  return self.Text, nil
}

func VariableResToString(res VariableType) (string, error) {
  switch res.Type {
  case PY_TYPE_STRING:
    if v, ok := res.Data.(string); !ok {
      return "", errors.New("error converting string variable result to a string")
    } else {
      return v, nil
    }
  case PY_TYPE_INT:
    if v, ok := res.Data.(int64); !ok {
      return "", errors.New("error converting integer variable result to a string")
    } else {
      return strconv.FormatInt(v, 10), nil
    }
  case PY_TYPE_BOOL:
    if v, ok := res.Data.(bool); !ok {
      return "", errors.New("error converting boolean variable result to a string")
    } else {
      return strconv.FormatBool(v), nil
    }
  case PY_TYPE_FLOAT:
    if v, ok := res.Data.(float64); !ok {
      return "", errors.New("error converting float variable result to a string")
    } else {
      return strconv.FormatFloat(v, 'f', -1, 64), nil
    }
  case PY_TYPE_LIST:
    if v, ok := res.Data.([]VariableType); !ok {
      return "", errors.New("error converting list variable result to a string")
    } else {
      res := "["
      for idx, item := range v {
        item_str, err := VariableResToString(item)
        if err != nil {
          return "", err
        }
        res += item_str
        if idx < len(v) - 1 {
          res += ", "
        }
      }
      res += "]"
      return res, nil
    }
  case PY_TYPE_DICT:
    if v, ok := res.Data.(map[VariableType]VariableType); !ok {
      return "", errors.New("error converting dict variable result to a string")
    } else {
      res := "{"
      cur := 0
      for key, val := range v {
        key_str, key_err := VariableResToString(key)
        if key_err != nil {
          return "", key_err
        }
        val_str, val_err := VariableResToString(val)
        if val_err != nil {
          return "", val_err
        }
        if key.Type == PY_TYPE_STRING {
          res += "'" + key_str + "'"
        } else {
          res += key_str
        }
        res = res + ": "
        if val.Type == PY_TYPE_STRING {
          res += "'" + val_str + "'"
        } else {
          res += val_str
        }
        if cur < len(v) - 1 {
          res += ", "
        }
        cur += 1
      }
      res += "}"
      return res, nil
    }
  }
  return "", errors.New("unknown type returned from variable statement ("+strconv.Itoa(int(res.Type))+"), cannot convert it to a string")
}
type VariableChunk struct {
  VarAst *VariableStatement
}
func (self *VariableChunk) Render(c *Context) (string, error) {
  res, err := self.VarAst.Eval(c)
  if err != nil {
    return "ERROR EVALUATING VARIABLE STATEMENT", err
  }
  return VariableResToString(res)
}

type IfChunk struct {
  IfAst *IfStatement
  IfChunks []Renderable
  ElifChunks []ElifChunk
  ElseChunks []Renderable
}
func (self *IfChunk) Render(c *Context) (string, error) {
  chunk_rendered := false
  res := ""
  v, err := self.IfAst.Eval(c)
  if err != nil {
    // FIXME: error handling
    return "ERROR EVALUATING IF STATEMENT", err
  }
  v_bool, ok := v.Data.(bool)
  if !ok {
    return "COULD NOT CONVERT IF RETURN TO A BOOLEAN", nil
  }
  if v_bool {
    for _, chunk := range self.IfChunks {
      c_res, err := chunk.Render(c)
      if err != nil {
        return "", err
      } else {
        res = res + c_res
      }
    }
    chunk_rendered = true
  } else {
    for _, elif := range self.ElifChunks {
      v, err := elif.ElifAst.Eval(c)
      if err != nil {
        // FIXME: error handling
        return "ERROR EVALUATING ELIF STATEMENT", nil
      }
      v_bool, ok := v.Data.(bool)
      if !ok {
        return "COULD NOT CONVERT IF RETURN TO A BOOLEAN IN ELIF", nil
      }
      if v_bool {
        for _, chunk := range elif.ElifChunks {
          c_res, err := chunk.Render(c)
          if err != nil {
            return "", err
          } else {
            res = res + c_res
          }
        }
        chunk_rendered = true
        break
      }
    }
  }
  if !chunk_rendered {
    for _, chunk := range self.ElseChunks {
      c_res, err := chunk.Render(c)
      if err != nil {
        return "", err
      } else {
        res = res + c_res
      }
    }
  }
  return res, nil
}

type ElifChunk struct {
  ElifAst    *ElifStatement
  ElifChunks []Renderable
}

type ForChunk struct {
  ForAst *ForStatement
  IfAst *IfStatement
  Chunks []Renderable
  ElseChunks []Renderable
}
func (self *ForChunk) Render(c *Context) (string, error) {
  if len(self.ForAst.TargetList.Targets) == 0 {
    return "ERROR EVALUATING FOR LOOP", errors.New("no targets found for assignment in the for loop")
  }
  res := ""
  did_loop := false
  num_tests := int64(len(self.ForAst.TestList.Tests))
  // FIXME: last_val would be used with the `loop.changed()` call,
  //        but that is not yet implemented (needs callables)
  //last_val := VariableType{PY_TYPE_UNDEFINED, nil}
  // FIXME: this is where in python you'd get an iterable type. It
  //        might be easier to abstract this as an iterable in the
  //        same way.
  loop_items := make([]VariableType, 0)
  if num_tests == 1 {
    test_res, err := self.ForAst.TestList.Tests[0].Eval(c)
    if err != nil {
      return "ERROR EVALUATING FOR LOOP", err
    }
    switch test_res.Type {
    // FIXME: handle other special cases
    case PY_TYPE_LIST:
      // use the list as the list of items
      v_list, _ := test_res.Data.([]VariableType)
      loop_items = append(loop_items, v_list...)
    case PY_TYPE_DICT:
      // use the list as the list of items
      v_list, _ := test_res.Data.(map[VariableType]VariableType)
      for k, v := range v_list {
        loop_items = append(loop_items, VariableType{PY_TYPE_LIST, []VariableType{k, v}})
      }
    case PY_TYPE_STRING:
      // make a loop item out of each character
      str, _ := test_res.AsString()
      for _, c := range str {
        loop_items = append(loop_items, VariableType{PY_TYPE_STRING, c})
      }
    default:
      // just use the result as the only item
      loop_items = append(loop_items, test_res)
    }
  } else if num_tests > 1 {
    for _, test := range self.ForAst.TestList.Tests {
      test_res, err := test.Eval(c)
      if err != nil {
        return "ERROR EVALUATING FOR LOOP", err
      }
      loop_items = append(loop_items, test_res)
    }
  }

  // reset the number of tests we're doing now
  num_tests = int64(len(loop_items))

  for idx, item := range loop_items {
    // set loop variables
    loop_vars := make(map[string]VariableType)
    loop_vars["index"] = VariableType{PY_TYPE_INT, int64(idx + 1)}
    loop_vars["index0"] = VariableType{PY_TYPE_INT, int64(idx)}
    loop_vars["revindex"] = VariableType{PY_TYPE_INT, num_tests - int64(idx + 1)} // FIXME: not sure if this is right
    loop_vars["revindex0"] = VariableType{PY_TYPE_INT, num_tests - int64(idx)}  // FIXME: not sure if this is right
    loop_vars["first"] = VariableType{PY_TYPE_BOOL, idx == 0}
    loop_vars["last"] = VariableType{PY_TYPE_BOOL, int64(idx) == num_tests}
    loop_vars["length"] = VariableType{PY_TYPE_BOOL, num_tests}
    loop_vars["depth"] = VariableType{PY_TYPE_BOOL, int64(1)} // FIXME: recursive property
    loop_vars["depth0"] = VariableType{PY_TYPE_BOOL, int64(0)} // FIXME: recursive property
    // FIXME: implement loop.cycle (callable)
    // FIXME: implement loop.changed (callable)
    // FIXME: implement loop() (callable)
    if idx > 0 {
      loop_vars["previtem"] = loop_items[idx - 1]
    } else {
      loop_vars["previtem"] = VariableType{PY_TYPE_UNDEFINED, nil}
    }
    if int64(idx) < num_tests - 1 {
      loop_vars["nextitem"] = loop_items[idx + 1]
    } else {
      loop_vars["nextitem"] = VariableType{PY_TYPE_UNDEFINED, nil}
    }
    c.Variables["loop"] = VariableType{PY_TYPE_DICT, loop_vars}
    // map the test result to the expression list
    target_len := len(self.ForAst.TargetList.Targets)
    if target_len != 1 {
      if item.Type != PY_TYPE_LIST {
        return "Assignment Error", errors.New("Cannot assign a single value to multiple targets.")
      }
      v_list, _ := item.Data.([]VariableType)
      item_len := len(v_list)
      if item_len != target_len {
        return "Assignment Error", errors.New("Cannot assign "+strconv.Itoa(item_len)+" values to "+strconv.Itoa(target_len)+" targets.")
      } else {
        for idx, target := range self.ForAst.TargetList.Targets {
          c.Variables[*target.Name] = v_list[idx]
        }
      }
    } else {
      target := self.ForAst.TargetList.Targets[0]
      c.Variables[*target.Name] = item
    }
    do_loop := true
    if self.ForAst.IfStatement != nil {
      if_res, err := self.ForAst.IfStatement.Eval(c)
      if err != nil {
        return "ERROR EVALUATING IF STATEMENT ON LOOP", err
      }
      if_bool, err := if_res.AsBool()
      if err != nil {
        return "ERROR CONVERTING IF RESULT ON LOOP TO BOOLEAN RESULT", err
      }
      do_loop = if_bool
    }
    if do_loop {
      // render the main chunks
      for _, chunk := range self.Chunks {
        c_res, err := chunk.Render(c)
        //print("CRES IS: "+c_res+"\n")
        if err != nil {
          return "", err
        } else {
          res = res + c_res
        }
        // mark the loop flag as true so we don't execute the else statement
        did_loop = true
      }
    }
    // cleanup the loop variables from the context
    delete(c.Variables, "loop")
  }
  if !did_loop {
    // render the else chunks
    for _, chunk := range self.ElseChunks {
      c_res, err := chunk.Render(c)
      if err != nil {
        return "", err
      } else {
        res = res + c_res
      }
    }
  }
  return res, nil
}

type RawChunk struct {
  Content string
}
func (self *RawChunk) Render(c *Context) (string, error) {
  return self.Content, nil
}

func ParseBlocks(tokens []Token, pos int, inside string) (int, []Renderable, error) {
  //fmt.Println("PARSING BLOCKS", pos)
  var contained_chunks []Renderable
  cur_pos := pos
  stop_parsing := false
  for cur_pos < len(tokens) {
    res := PeekToken(tokens[cur_pos])
    switch res {
    case "text":
      new_pos, text_chunk, err := ParseText(tokens, cur_pos)
      if err != nil {
        return cur_pos, nil, err
      }
      contained_chunks = append(contained_chunks, text_chunk)
      cur_pos = new_pos
    case "variable":
      new_pos, var_chunk, err := ParseVariable(tokens, cur_pos)
      if err != nil {
        return cur_pos, nil, err
      }
      contained_chunks = append(contained_chunks, var_chunk)
      cur_pos = new_pos
    case "if":
      new_pos, if_chunk, err := ParseIf(tokens, cur_pos)
      if err != nil {
        return cur_pos, nil, err
      }
      contained_chunks = append(contained_chunks, if_chunk)
      cur_pos = new_pos
    case "for":
      new_pos, for_chunk, err := ParseFor(tokens, cur_pos)
      if err != nil {
        return cur_pos, nil, err
      }
      contained_chunks = append(contained_chunks, for_chunk)
      cur_pos = new_pos
    case "elif", "endif":
      if inside == "if" {
        stop_parsing = true
      } else {
        return cur_pos, nil, errors.New("invalid token found: '" + res + "' but not currently inside an if statement")
      }
    case "endfor":
      if inside == "for" {
        stop_parsing = true
      } else {
        return cur_pos, nil, errors.New("invalid token found: '" + res + "' but not currently inside a for statement")
      }
    case "else":
      if inside == "for" || inside == "if" {
        stop_parsing = true
      } else {
        return cur_pos, nil, errors.New("invalid token found: '" + res + "' but not currently inside an if or a for statement")
      }
    case "raw":
      new_pos, raw_chunk, err := ParseRaw(tokens, cur_pos)
      if err != nil {
        return cur_pos, nil, err
      }
      contained_chunks = append(contained_chunks, raw_chunk)
      cur_pos = new_pos
    default:
      return cur_pos, nil, errors.New("invalid token found: '" + res + "'")
    }
    if stop_parsing { break }
  }
  //fmt.Println("DONE PARSING BLOCKS", cur_pos)
  return cur_pos, contained_chunks, nil
}
func ParseRaw(tokens []Token, pos int) (int, Renderable, error) {
  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "raw" {
    return cur_pos, &DummyChunk{}, errors.New("expected a raw token, found '" + res + "' instead")
  }
  raw_token := tokens[cur_pos].(RawToken)
  cur_pos += 1
  if res := PeekToken(tokens[cur_pos]); res != "endraw" {
    return cur_pos, &DummyChunk{}, errors.New("expected a endraw token, found '" + res + "' instead")
  }
  raw_chunk := new(RawChunk)
  raw_chunk.Content = raw_token.Content
  return cur_pos+1, raw_chunk, nil
}
func ParseText(tokens []Token, pos int) (int, Renderable, error) {
  //fmt.Println("PARSING TEXT", pos)
  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "text" {
    return cur_pos, &DummyChunk{}, errors.New("expected a text token, found '" + res + "' instead")
  } else {
    //fmt.Println("DONE PARSING TEXT")
    text_token := tokens[cur_pos].(TextToken)
    text_chunk := new(TextChunk)
    text_chunk.Text = text_token.Text
    return cur_pos+1, text_chunk, nil
  }
}
func ParseVariableStatement(statement string) (*VariableStatement, error) {
  //fmt.Println("PARSING VARIABLE STATEMENT:", statement)
  parser, err := participle.Build(&VariableStatement{}, PythonLexer)
  if err != nil {
    return nil, err
  }
  ast := &VariableStatement{}
  if err := parser.ParseString(statement, ast); err != nil {
    return nil, err
  }
  //fmt.Println("DONE PARSING VARIABLE STATEMENT")
  return ast, nil
}
func ParseVariable(tokens []Token, pos int) (int, Renderable, error) {
  //fmt.Println("PARSING VARIABLE", pos)
  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "variable" {
    return cur_pos, &DummyChunk{}, errors.New("expected a variable token, found '" + res + "' instead")
  } else {
    //fmt.Println("DONE PARSING VARIABLE")
    var_token := tokens[cur_pos].(VariableToken)
    var_chunk := new(VariableChunk)
    ast, err := ParseVariableStatement(var_token.Content)
    if err != nil {
      return pos, &DummyChunk{}, err
    }
    var_chunk.VarAst = ast
    return cur_pos+1, var_chunk, nil
  }
}

func ParseIfStatement(statement string) (*IfStatement, error) {
  //fmt.Println("PARSING IF:", statement)
  parser, err := participle.Build(&IfStatement{}, PythonLexer)
  if err != nil {
    return nil, err
  }
  ast := &IfStatement{}
  if err := parser.ParseString(statement, ast); err != nil {
    return nil, err
  }
  return ast, nil
}
func ParseIf(tokens []Token, pos int) (int, Renderable, error) {
  //fmt.Println("PARSING IF", pos, "Number of Tokens:", len(tokens))
  if_chunk := new(IfChunk)
  if_chunk.IfAst = nil
  if_chunk.IfChunks = make([]Renderable, 0)
  if_chunk.ElifChunks = make([]ElifChunk, 0)
  if_chunk.ElseChunks = make([]Renderable, 0)

  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "if" {
    return cur_pos, &DummyChunk{}, errors.New("expected an 'if' token but got '" + res + "' instead.")
  }

  if_token := tokens[cur_pos].(IfToken)
  ast, err := ParseIfStatement(if_token.IfStatement)
  if err != nil {
    return pos, &DummyChunk{}, err
  }
  if_chunk.IfAst = ast
  cur_pos += 1

  found_endif := false
  for cur_pos < len(tokens) {
    res := PeekToken(tokens[cur_pos])
    switch res {
    case "endif":
      found_endif = true
      cur_pos += 1
    case "elif":
      new_pos, elif_chunk, err := ParseElif(tokens, cur_pos)
      if err != nil {
        return cur_pos, &DummyChunk{}, err
      }
      if_chunk.ElifChunks = append(if_chunk.ElifChunks, elif_chunk)
      cur_pos = new_pos
    case "else":
      new_pos, else_chunks, err := ParseElse(tokens, cur_pos, "if")
      if err != nil {
        return cur_pos, &DummyChunk{}, err
      }
      if_chunk.ElseChunks = append(if_chunk.ElseChunks, else_chunks...)
      cur_pos = new_pos
    default:
      new_pos, contained_chunks, err := ParseBlocks(tokens, cur_pos, "if")
      if err != nil {
        return cur_pos, &DummyChunk{}, err
      }
      if_chunk.IfChunks = append(if_chunk.IfChunks, contained_chunks...)
      cur_pos = new_pos
    }
    if found_endif { break }
  }
  if !found_endif {
    //fmt.Println("NO ENDIF!!!!")
    return cur_pos, &DummyChunk{}, errors.New("Missing 'endif' for if statement tag.")
  }
  //fmt.Println("DONE PARSING IF STATEMENT")
  return cur_pos, if_chunk, nil
}

func ParseElifStatement(statement string) (*ElifStatement, error) {
  parser, err := participle.Build(&ElifStatement{}, PythonLexer)
  if err != nil {
    return nil, err
  }
  ast := &ElifStatement{}
  if err := parser.ParseString(statement, ast); err != nil {
    return nil, err
  }
  return ast, nil
}
func ParseElif(tokens []Token, pos int) (int, ElifChunk, error) {
  //fmt.Println("PARSING ELIF STATEMENT", pos)
  elif_chunk := new(ElifChunk)
  elif_chunk.ElifAst = nil
  elif_chunk.ElifChunks = make([]Renderable, 0)

  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "elif" {
    return cur_pos, *elif_chunk, errors.New("expected an 'elif' token but got '" + res + "' instead.")
  }
  elif_token := tokens[cur_pos].(ElifToken)
  ast, err := ParseElifStatement(elif_token.ElifStatement)
  if err != nil {
    return pos, *elif_chunk, err
  }
  elif_chunk.ElifAst = ast
  cur_pos += 1

  found_stop := false
  for cur_pos < len(tokens) {
    res := PeekToken(tokens[cur_pos])
    switch res {
    case "elif", "else", "endif":
      found_stop = true
    default:
      new_pos, contained_chunks, err := ParseBlocks(tokens, cur_pos, "if")
      if err != nil {
        return cur_pos, *elif_chunk, err
      }
      elif_chunk.ElifChunks = append(elif_chunk.ElifChunks, contained_chunks...)
      cur_pos = new_pos
    }
    if found_stop { break }
  }
  //fmt.Println("DONE PARSING ELIF STATEMENT")
  return cur_pos, *elif_chunk, nil
}

func ParseElse(tokens []Token, pos int, in string) (int, []Renderable, error) {
  //fmt.Println("PARSING ELSE STATEMENT", pos)
  chunks := make([]Renderable, 0)
  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "else" {
    return cur_pos, chunks, errors.New("expected an 'else' token but got '" + res + "' instead.")
  }
  cur_pos += 1

  found_stop := false
  for cur_pos < len(tokens) {
    res := PeekToken(tokens[cur_pos])
    switch res {
    case "endfor":
      if in == "for" {
        found_stop = true
      } else {
        return cur_pos, chunks, errors.New("unexpected 'endfor' found when not in a for loop.")
      }
    case "endif":
      if in == "if" {
        found_stop = true
      } else {
        return cur_pos, chunks, errors.New("unexpected 'endif' found when not in an if statement.")
      }
    default:
      new_pos, contained_chunks, err := ParseBlocks(tokens, cur_pos, in)
      if err != nil {
        return cur_pos, chunks, err
      }
      chunks = append(chunks, contained_chunks...)
      cur_pos = new_pos
    }
    if found_stop { break }
  }
  //fmt.Println("DONE PARSING ELSE STATEMENT")
  return cur_pos, chunks, nil
}
func ParseForStatement(statement string) (*ForStatement, error) {
  //fmt.Println("PARSING FOR STATEMENT:", statement)
  parser, err := participle.Build(&ForStatement{}, PythonLexer)
  if err != nil {
    return nil, err
  }
  ast := &ForStatement{}
  if err := parser.ParseString(statement, ast); err != nil {
    return nil, err
  }
  //fmt.Println("DONE PARSING FOR STATEMENT")
  return ast, nil
}
func ParseFor(tokens []Token, pos int) (int, Renderable, error) {
  //fmt.Println("PARSING FOR STATEMENT", pos)
  for_chunk := new(ForChunk)
  for_chunk.ForAst = nil
  for_chunk.Chunks = make([]Renderable, 0)
  for_chunk.ElseChunks = make([]Renderable, 0)

  cur_pos := pos
  if res := PeekToken(tokens[cur_pos]); res != "for" {
    return cur_pos, &DummyChunk{}, errors.New("expected a 'for' token but got '" + res + "' instead.")
  }
  for_token := tokens[cur_pos].(ForToken)
  ast, err := ParseForStatement(for_token.ForStatement)
  if err != nil {
    return pos, &DummyChunk{}, err
  }
  for_chunk.ForAst = ast
  cur_pos += 1

  found_endfor := false
  for cur_pos < len(tokens) {
    res := PeekToken(tokens[cur_pos])
    switch res {
    case "endfor":
      found_endfor = true
      cur_pos += 1
    case "else":
      new_pos, else_chunks, err := ParseElse(tokens, cur_pos, "for")
      if err != nil {
        return cur_pos, &DummyChunk{}, err
      }
      for_chunk.ElseChunks = append(for_chunk.ElseChunks, else_chunks...)
      cur_pos = new_pos
    default:
      new_pos, contained_chunks, err := ParseBlocks(tokens, cur_pos, "for")
      if err != nil {
        return cur_pos, &DummyChunk{}, err
      }
      for_chunk.Chunks = append(for_chunk.Chunks, contained_chunks...)
      cur_pos = new_pos
    }
    if found_endfor { break }
  }
  if !found_endfor {
    return cur_pos, &DummyChunk{}, errors.New("Missing matching 'endfor' for a 'for' loop tag.")
  }
  return cur_pos, for_chunk, nil
}
