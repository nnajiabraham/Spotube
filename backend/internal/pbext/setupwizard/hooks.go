package setupwizard

import (
	// "net/http"

	// "github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	// "github.com/pocketbase/pocketbase/core"
)

// RegisterHooks registers hooks to prevent direct access to settings collection
func RegisterHooks(app *pocketbase.PocketBase) {
	// Deny listing settings records
	// app.OnRecordsListRequest("settings").Add(func(e *core.RecordsListEvent) error {
	// 	return echo.NewHTTPError(http.StatusForbidden, "Direct access to settings collection is not allowed")
	// })

	// // Deny viewing individual settings records
	// app.OnRecordViewRequest("settings").Add(func(e *core.RecordViewEvent) error {
	// 	return echo.NewHTTPError(http.StatusForbidden, "Direct access to settings collection is not allowed")
	// })

	// // Deny creating settings records via API (should only be done via setup wizard)
	// app.OnRecordBeforeCreateRequest("settings").Add(func(e *core.RecordCreateEvent) error {
	// 	return echo.NewHTTPError(http.StatusForbidden, "Settings records can only be created via setup wizard")
	// })

	// // Deny updating settings records via API (should only be done via setup wizard)
	// app.OnRecordBeforeUpdateRequest("settings").Add(func(e *core.RecordUpdateEvent) error {
	// 	return echo.NewHTTPError(http.StatusForbidden, "Settings records can only be updated via setup wizard")
	// })

	// // Deny deleting settings records via API
	// app.OnRecordBeforeDeleteRequest("settings").Add(func(e *core.RecordDeleteEvent) error {
	// 	return echo.NewHTTPError(http.StatusForbidden, "Settings records cannot be deleted")
	// })
}
