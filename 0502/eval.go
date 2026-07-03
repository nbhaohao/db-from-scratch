package db0502

// 你来实现（递归遍历表达式树求值 = tree-walk interpreter）：
//
//	switch e := expr.(type) 分三种：
//	 - string（列名）：在 schema.Cols 里按名字找下标，返回 &row[idx]（该列的值）；找不到报 unknown column
//	 - *Cell（字面量）：直接返回 e
//	 - *ExprBinOp（运算）：先递归 evalExpr 左、右子树；若 left.Type != right.Type 报 type mismatch，
//	     再按 (e.op, 类型) 分派：OP_ADD+字符串→拼接(slices.Concat)、OP_ADD+整数→加、OP_SUB+整数→减，其余报 bad binary op
//	 - default：panic("unreachable")——非法类型是代码 bug，不是数据错误
func evalExpr(schema *Schema, row Row, expr interface{}) (*Cell, error)

// UzBVUkNF https://systems-programming.org/
