package router

import (
	"github.com/go-chi/chi/v5"

	authHandlers "github.com/sliitmozilla/accounts/app/handlers"
	connectionHandler "github.com/sliitmozilla/accounts/app/handlers/connections"
	"github.com/sliitmozilla/accounts/app/middlewares"
)

type AuthRoutes struct{}

func (b AuthRoutes) Routes() chi.Router {

	r := chi.NewRouter()

	r.Route("/", func(authRoutes chi.Router) {
		authRoutes.Get("/authorize", authHandlers.Authorize)
		authRoutes.Get("/authorize/confirm", authHandlers.AuthorizeConfirm)
		authRoutes.With(middlewares.AuthHandler).With(middlewares.RequireLogin).Get("/session", authHandlers.GetSession)
		authRoutes.Post("/login", authHandlers.Login)
		authRoutes.Post("/logout", authHandlers.Logout)
		authRoutes.Route("/token", func(authTokenRoutes chi.Router) {
			authTokenRoutes.Post("/", authHandlers.GetToken)
			authTokenRoutes.Post("/refresh", authHandlers.RefreshToken)
		})

		authRoutes.Route("/google", func(googleAuth chi.Router) {
			googleAuth.With(middlewares.AuthHandler).Get("/url", connectionHandler.GetGoogleAuthURL)
			googleAuth.With(middlewares.AuthHandler).Post("/url", connectionHandler.GetGoogleAuthURL)
			googleAuth.Get("/callback", connectionHandler.CallbackGoogle)
			googleAuth.Post("/callback", connectionHandler.CallbackGoogle)
		})

		authRoutes.Route("/auth/google", func(googleAuth chi.Router) {
			googleAuth.With(middlewares.AuthHandler).Get("/url", connectionHandler.GetGoogleAuthURL)
			googleAuth.With(middlewares.AuthHandler).Post("/url", connectionHandler.GetGoogleAuthURL)
			googleAuth.Get("/callback", connectionHandler.CallbackGoogle)
			googleAuth.Post("/callback", connectionHandler.CallbackGoogle)
		})
	})

	return r
}
