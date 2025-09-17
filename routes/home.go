package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/schema"
	conf "github.com/muety/wakapi/config"
	"github.com/muety/wakapi/middlewares"
	"github.com/muety/wakapi/models/view"
	routeutils "github.com/muety/wakapi/routes/utils"
	"github.com/muety/wakapi/services"
)

type HomeHandler struct {
	config       *conf.Config
	userSrvc     services.IUserService
	keyValueSrvc services.IKeyValueService
}

var loginDecoder = schema.NewDecoder()
var loginTOTPDecoder = schema.NewDecoder()
var signupDecoder = schema.NewDecoder()
var resetPasswordDecoder = schema.NewDecoder()

func NewHomeHandler(userService services.IUserService, keyValueService services.IKeyValueService) *HomeHandler {
	return &HomeHandler{
		config:       conf.Get(),
		userSrvc:     userService,
		keyValueSrvc: keyValueService,
	}
}

func (h *HomeHandler) RegisterRoutes(router chi.Router) {
	router.Group(func(r chi.Router) {
		r.Use(middlewares.NewAuthenticateMiddleware(h.userSrvc).WithOptionalFor("/").Handler)
		r.Get("/", h.GetIndex)
	})
}

func (h *HomeHandler) GetIndex(w http.ResponseWriter, r *http.Request) {
	if h.config.IsDev() {
		loadTemplates()
	}

	if user := middlewares.GetPrincipal(r); user != nil {
		http.Redirect(w, r, fmt.Sprintf("%s/summary", h.config.Server.BasePath), http.StatusFound)
		return
	}
	if h.config.Security.DisableFrontpage {
		http.Redirect(w, r, fmt.Sprintf("%s/login", h.config.Server.BasePath), http.StatusFound)
		return
	}

	templates[conf.IndexTemplate].Execute(w, h.buildViewModel(r, w))
}

func (h *HomeHandler) buildViewModel(r *http.Request, w http.ResponseWriter) *view.HomeViewModel {
	var (
		totalHours      int
		totalUsers      int
		currentlyOnline int
		newsbox         view.Newsbox
	)

	if kv, err := h.keyValueSrvc.GetString(conf.KeyLatestTotalTime); err == nil && kv != nil && kv.Value != "" {
		if d, err := time.ParseDuration(kv.Value); err == nil {
			totalHours = int(d.Hours())
		}
	}

	if kv, err := h.keyValueSrvc.GetString(conf.KeyLatestTotalUsers); err == nil && kv != nil && kv.Value != "" {
		if d, err := strconv.Atoi(kv.Value); err == nil {
			totalUsers = d
		}
	}

	if kv, err := h.keyValueSrvc.GetString(conf.KeyNewsbox); err == nil && kv != nil && kv.Value != "" {
		if err := json.NewDecoder(strings.NewReader(kv.Value)).Decode(&newsbox); err != nil {
			conf.Log().Request(r).Error("failed to decode newsbox message", "error", err)
		}
	}

	if c, err := h.userSrvc.CountCurrentlyOnline(); err == nil {
		currentlyOnline = c
	} else {
		conf.Log().Request(r).Error("failed to count currently online users", "error", err)
	}

	sharedVm := view.NewSharedViewModel(h.config, nil)
	sharedVm.LeaderboardEnabled = sharedVm.LeaderboardEnabled && !h.config.App.LeaderboardRequireAuth // logged in users will never actually see the home route's view
	vm := &view.HomeViewModel{
		SharedViewModel: sharedVm,
		TotalHours:      totalHours,
		TotalUsers:      totalUsers,
		CurrentlyOnline: currentlyOnline,
		Newsbox:         &newsbox,
	}
	return routeutils.WithSessionMessages(vm, r, w)
}
