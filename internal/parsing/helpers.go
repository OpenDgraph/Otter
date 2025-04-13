package parsing

func cleanNulls(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		clean := make(map[string]interface{})
		for k, v2 := range val {
			c := cleanNulls(v2)

			switch c := c.(type) {
			case nil:
				continue
			case string:
				if c == "" {
					continue
				}
			case bool:
				if !c {
					continue
				}
			case []interface{}:
				if len(c) == 0 {
					continue
				}
			case map[string]interface{}:
				if len(c) == 0 {
					continue
				}
			}
			clean[k] = c
		}
		return clean

	case []interface{}:
		var out []interface{}
		for _, v2 := range val {
			c := cleanNulls(v2)
			if c != nil {
				out = append(out, c)
			}
		}
		return out
	default:
		return val
	}
}

func removeEmptyVars(ast map[string]interface{}) {
	if qvRaw, ok := ast["QueryVars"]; ok {
		if qvSlice, ok := qvRaw.([]interface{}); ok {
			allEmpty := true
			for _, v := range qvSlice {
				if m, ok := v.(map[string]interface{}); !ok || len(m) > 0 {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				delete(ast, "QueryVars")
			}
		}
	}
}
