package router

import (
	"github.com/gin-gonic/gin"

	"radiation-dose-budget-control/backend/internal/constants"
	"radiation-dose-budget-control/backend/internal/handler"
	"radiation-dose-budget-control/backend/internal/middleware"
)

func registerControlMeasureRoutes(group *gin.RouterGroup, measures *handler.ControlMeasureHandler, scenarios *handler.BudgetScenarioHandler) {
	measureRoutes := group.Group("/measures")
	measureRoutes.GET("", measures.List)
	measureRoutes.GET("/:id", measures.Get)
	measureRoutes.POST("", middleware.RBAC(constants.RolePlanner, constants.RoleAdmin), measures.Create)
	measureRoutes.PUT("/:id", middleware.RBAC(constants.RolePlanner, constants.RoleAdmin), measures.Update)
	measureRoutes.POST("/:id/status", middleware.RBAC(constants.RolePlanner, constants.RoleAdmin), measures.SetStatus)

	scenarioRoutes := group.Group("/scenarios")
	scenarioRoutes.GET("", scenarios.List)
	scenarioRoutes.GET("/:id", scenarios.Get)
	scenarioRoutes.POST("", middleware.RBAC(constants.RolePlanner, constants.RoleAdmin), scenarios.Create)
}
