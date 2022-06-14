package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gitlab.com/demodesk/neko/server/internal/api/members"
	"gitlab.com/demodesk/neko/server/internal/api/room"
	"gitlab.com/demodesk/neko/server/pkg/auth"
	"gitlab.com/demodesk/neko/server/pkg/types"
	"gitlab.com/demodesk/neko/server/pkg/utils"
)

type ApiManagerCtx struct {
	sessions types.SessionManager
	members  types.MemberManager
	desktop  types.DesktopManager
	capture  types.CaptureManager
	routers  map[string]func(types.Router)
}

func New(
	sessions types.SessionManager,
	members types.MemberManager,
	desktop types.DesktopManager,
	capture types.CaptureManager,
) *ApiManagerCtx {

	return &ApiManagerCtx{
		sessions: sessions,
		members:  members,
		desktop:  desktop,
		capture:  capture,
		routers:  make(map[string]func(types.Router)),
	}
}

func (api *ApiManagerCtx) Route(r types.Router) {
	r.Post("/login", api.Login)

	// Authenticated area
	r.Group(func(r types.Router) {
		r.Use(api.Authenticate)

		r.Post("/logout", api.Logout)
		r.Get("/whoami", api.Whoami)

		membersHandler := members.New(api.members)
		r.Route("/members", membersHandler.Route)
		r.Route("/members_bulk", membersHandler.RouteBulk)

		roomHandler := room.New(api.sessions, api.desktop, api.capture)
		r.Route("/room", roomHandler.Route)

		for path, router := range api.routers {
			r.Route(path, router)
		}
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) error {
		_, err := w.Write([]byte("true"))
		return err
	})

	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) error {
		promhttp.Handler().ServeHTTP(w, r)
		return nil
	})
}

func (api *ApiManagerCtx) Authenticate(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	session, err := api.sessions.Authenticate(r)
	if err != nil {
		if api.sessions.CookieEnabled() {
			api.sessions.CookieClearToken(w, r)
		}

		if errors.Is(err, types.ErrSessionLoginDisabled) {
			return nil, utils.HttpForbidden("login is disabled for this session")
		}

		return nil, utils.HttpUnauthorized().WithInternalErr(err)
	}

	return auth.SetSession(r, session), nil
}

func (api *ApiManagerCtx) AddRouter(path string, router func(types.Router)) {
	api.routers[path] = router
}
