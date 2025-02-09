/*
 * Access API
 *
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * API version: 1.0.0
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package generated

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/v1/",
		Index,
	},

	Route{
		"AccountsAddressGet",
		strings.ToUpper("Get"),
		"/v1/accounts/{address}",
		AccountsAddressGet,
	},

	Route{
		"BlocksGet",
		strings.ToUpper("Get"),
		"/v1/blocks",
		BlocksGet,
	},

	Route{
		"BlocksIdGet",
		strings.ToUpper("Get"),
		"/v1/blocks/{id}",
		BlocksIdGet,
	},

	Route{
		"CollectionsIdGet",
		strings.ToUpper("Get"),
		"/v1/collections/{id}",
		CollectionsIdGet,
	},

	Route{
		"ExecutionResultsGet",
		strings.ToUpper("Get"),
		"/v1/execution_results",
		ExecutionResultsGet,
	},

	Route{
		"ExecutionResultsIdGet",
		strings.ToUpper("Get"),
		"/v1/execution_results/{id}",
		ExecutionResultsIdGet,
	},

	Route{
		"ScriptsPost",
		strings.ToUpper("Post"),
		"/v1/scripts",
		ScriptsPost,
	},

	Route{
		"TransactionResultsTransactionIdGet",
		strings.ToUpper("Get"),
		"/v1/transaction_results/{transaction_id}",
		TransactionResultsTransactionIdGet,
	},

	Route{
		"TransactionsIdGet",
		strings.ToUpper("Get"),
		"/v1/transactions/{id}",
		TransactionsIdGet,
	},

	Route{
		"TransactionsPost",
		strings.ToUpper("Post"),
		"/v1/transactions",
		TransactionsPost,
	},
}
