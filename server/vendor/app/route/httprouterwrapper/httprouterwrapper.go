package httprouterwrapper

import (
"net/http"

"github.com/gorilla/context"
"github.com/julienschmidt/httprouter"
)

func Handler(h http.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		context.Set(r, "params", p)
		h.ServeHTTP(w, r)
	}
}
