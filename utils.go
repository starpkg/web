package web

import (
	"fmt"
	"net/http"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// Common type conversion helpers

// starlarkListToStringSlice converts a Starlark list to a Go string slice
func starlarkListToStringSlice(list *starlark.List) []string {
	if list == nil {
		return nil
	}

	slice := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		if str, ok := list.Index(i).(starlark.String); ok {
			slice[i] = str.GoString()
		}
	}
	return slice
}

// starlarkListToIntSlice converts a Starlark list to a Go int slice
func starlarkListToIntSlice(list *starlark.List) []int {
	if list == nil {
		return nil
	}

	slice := make([]int, list.Len())
	for i := 0; i < list.Len(); i++ {
		if intVal, ok := list.Index(i).(starlark.Int); ok {
			if val, ok := intVal.Int64(); ok {
				slice[i] = int(val)
			}
		}
	}
	return slice
}

// starlarkDictToStringMap converts a Starlark dict to a Go map[string]string
func starlarkDictToStringMap(dict *starlark.Dict) map[string]string {
	if dict == nil {
		return nil
	}

	result := make(map[string]string)
	iter := dict.Iterate()
	defer iter.Done()

	var k starlark.Value
	for iter.Next(&k) {
		v, _, err := dict.Get(k)
		if err != nil {
			continue
		}

		keyStr := dataconv.StarString(k)
		valueStr := dataconv.StarString(v)

		if keyStr != "" {
			result[keyStr] = valueStr
		}
	}

	return result
}

// starlarkDictToHeaders converts a Starlark dict to HTTP headers
func starlarkDictToHeaders(dict *starlark.Dict) map[string][]string {
	if dict == nil {
		return nil
	}

	headers := make(map[string][]string)
	iter := dict.Iterate()
	defer iter.Done()

	var k starlark.Value
	for iter.Next(&k) {
		v, _, err := dict.Get(k)
		if err != nil {
			continue
		}

		keyStr := dataconv.StarString(k)
		valueStr := dataconv.StarString(v)

		if keyStr != "" && valueStr != "" {
			headers[keyStr] = []string{valueStr}
		}
	}

	return headers
}

// createMultiValueDict creates a Starlark dict from multi-value data (like headers or form data)
func createMultiValueDict(data map[string][]string) *starlark.Dict {
	dict := starlark.NewDict(len(data))
	for name, values := range data {
		if len(values) == 1 {
			dict.SetKey(starlark.String(name), starlark.String(values[0]))
		} else {
			valueList := make([]starlark.Value, len(values))
			for i, v := range values {
				valueList[i] = starlark.String(v)
			}
			dict.SetKey(starlark.String(name), starlark.NewList(valueList))
		}
	}
	return dict
}

// Response error helpers

// createErrorResponse creates a standardized error response
func createErrorResponse(statusCode int, message string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers:    make(http.Header),
		Body:       message,
	}
}

// createJSONErrorResponse creates a JSON error response
func createJSONErrorResponse(statusCode int, message string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		JSONData:   map[string]interface{}{"error": message},
	}
}

// createHTMLErrorResponse creates an HTML error response
func createHTMLErrorResponse(statusCode int, message string) *Response {
	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Error %d</title>
</head>
<body>
    <h1>Error %d</h1>
    <p>%s</p>
</body>
</html>`, statusCode, statusCode, message)

	return &Response{
		StatusCode: statusCode,
		Headers:    map[string][]string{"Content-Type": {"text/html"}},
		Body:       htmlContent,
	}
}

// Common HTTP status code helpers

// BadRequest creates a 400 Bad Request response
func BadRequest(message string) *Response {
	if message == "" {
		message = "Bad Request"
	}
	return createErrorResponse(400, message)
}

// Unauthorized creates a 401 Unauthorized response
func Unauthorized(message string) *Response {
	if message == "" {
		message = "Unauthorized"
	}
	return createErrorResponse(401, message)
}

// Forbidden creates a 403 Forbidden response
func Forbidden(message string) *Response {
	if message == "" {
		message = "Forbidden"
	}
	return createErrorResponse(403, message)
}

// NotFound creates a 404 Not Found response
func NotFound(message string) *Response {
	if message == "" {
		message = "Not Found"
	}
	return createErrorResponse(404, message)
}

// MethodNotAllowed creates a 405 Method Not Allowed response
func MethodNotAllowed(message string) *Response {
	if message == "" {
		message = "Method Not Allowed"
	}
	return createErrorResponse(405, message)
}

// InternalServerError creates a 500 Internal Server Error response
func InternalServerError(message string) *Response {
	if message == "" {
		message = "Internal Server Error"
	}
	return createErrorResponse(500, message)
}

// JSON error response helpers

// BadRequestJSON creates a 400 Bad Request JSON response
func BadRequestJSON(message string) *Response {
	if message == "" {
		message = "Bad Request"
	}
	return createJSONErrorResponse(400, message)
}

// UnauthorizedJSON creates a 401 Unauthorized JSON response
func UnauthorizedJSON(message string) *Response {
	if message == "" {
		message = "Unauthorized"
	}
	return createJSONErrorResponse(401, message)
}

// ForbiddenJSON creates a 403 Forbidden JSON response
func ForbiddenJSON(message string) *Response {
	if message == "" {
		message = "Forbidden"
	}
	return createJSONErrorResponse(403, message)
}

// NotFoundJSON creates a 404 Not Found JSON response
func NotFoundJSON(message string) *Response {
	if message == "" {
		message = "Not Found"
	}
	return createJSONErrorResponse(404, message)
}

// InternalServerErrorJSON creates a 500 Internal Server Error JSON response
func InternalServerErrorJSON(message string) *Response {
	if message == "" {
		message = "Internal Server Error"
	}
	return createJSONErrorResponse(500, message)
}
