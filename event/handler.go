/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"fmt"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"go.uber.org/fx"
)

type Handler struct {
	Event http.Handler `name:"eventHandler"`
}

// NewHandlers builds handlers from endpoints and other input provided.
func NewHandlers(in EndpointsDecodeIn) Handler {
	return Handler{
		Event: NewEventHandler(in.Event, in.GetLogger),
	}
}

func NewEventHandler(e endpoint.Endpoint, getLogger GetLoggerFunc) http.Handler {
	return kithttp.NewServer(
		e,
		DecodeEvent,
		EncodeResponseCode(http.StatusOK),
		kithttp.ServerErrorEncoder(EncodeError(getLogger)),
	)
}

// RoutesIn provides the information needed to set up the router and start
// handling requests for glaukos's primary subscribing endpoint.
type RoutesIn struct {
	fx.In
	Handler Handler
	//AuthChain alice.Chain
	Router  *mux.Router `name:"servers.primary"`
	APIBase string      `name:"api_base"`
}

// ConfigureRoutes sets up the router provided to handle traffic for the events parsing endpoint
func ConfigureRoutes(in RoutesIn) {
	path := fmt.Sprintf("/%s/events", in.APIBase)
	//in.Router.Use(in.AuthChain.Then)
	in.Router.Handle(path, in.Handler.Event).
		Name("events").
		Methods("POST")
}
