package router

import (
	"github.com/go-chi/chi/v5"

	usersHandler "github.com/sliitmozilla/accounts/app/handlers"
	"github.com/sliitmozilla/accounts/app/middlewares"
)

type UsersRoute struct{}

func (b UsersRoute) Routes() chi.Router {

	r := chi.NewRouter()

	r.With(middlewares.AuthHandler).With(middlewares.RequireRoles("admin")).Get("/", usersHandler.GetUsers)
	r.Post("/", usersHandler.CreateUser)
	r.Get("/verify", usersHandler.VerifyUserEmail)
	r.Post("/verify", usersHandler.VerifyUserEmail)
	r.Post("/resend-verification", usersHandler.ResendVerificationEmail)

	r.Route("/me", func(usersRoute chi.Router) {
		usersRoute.Use(middlewares.AuthHandler)
		usersRoute.Use(middlewares.RequireLogin)
		usersRoute.Get("/", usersHandler.GetMe)
		usersRoute.Patch("/", usersHandler.UpdateMe)
		usersRoute.Patch("/password", usersHandler.ChangePassword)
	})

	r.Route("/{id}", func(usersRoute chi.Router) {
		usersRoute.Use(middlewares.AuthHandler)
		usersRoute.Use(middlewares.RequireLogin)
		usersRoute.Get("/", usersHandler.GetUser)
		usersRoute.With(middlewares.RequireRoles(("admin"))).Patch("/", usersHandler.UpdateUser)
		usersRoute.With(middlewares.RequireRoles(("admin"))).Delete("/", usersHandler.DeleteUser)
		usersRoute.Route("/roles", func(usersRolesRoute chi.Router) {
			usersRolesRoute.Use(middlewares.RequireLogin)
			usersRolesRoute.Use(middlewares.RequireRoles("admin"))
			usersRolesRoute.Post("/", usersHandler.AddRole)
			usersRolesRoute.Delete("/{role}", usersHandler.RemoveRole)
		})
	})

	return r
}
