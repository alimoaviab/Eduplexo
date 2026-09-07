package auth

import "github.com/eduplexo/backend-go/internal/api"

// Action is one of the five permission verbs from
// old-app/shared/types/core.ts.
type Action string

const (
	ActionView   Action = "view"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionManage Action = "manage"
)

// Feature mirrors the `Feature` union from
// old-app/shared/types/core.ts.
type Feature string

// roleAccessMatrix is ported verbatim from
// old-app/shared/auth/rbac.ts. Each role maps to a set of features and the
// actions allowed on each feature. `manage` is treated as a superset of any
// other action by `CanAccess`.
var roleAccessMatrix = map[string]map[Feature][]Action{
	"super_admin": {
		"platform":   {ActionManage},
		"schools":    {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"audit_logs": {ActionView},
		"reports":    {ActionView},
	},
	// Owner is a retired role. It no longer exists in the access matrix: any
	// legacy claim carrying role='owner' fails every permission check, and the
	// auth layer refuses to issue such sessions.
	"admin": {
		"users":         {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"settings":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"students":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"teachers":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"subjects":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"classes":       {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"attendance":    {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"homework":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"exams":         {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"results":       {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"fees":          {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"reports":       {ActionView},
		"notifications": {ActionView, ActionCreate, ActionUpdate},
		"audit_logs":    {ActionView},
		"announcements": {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"timetable":     {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"behavior":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"leave":         {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"events":        {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"schedules":     {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"certificates":  {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
		"expenses":      {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionManage},
	},
	"teacher": {
		"settings":      {ActionView},
		"students":      {ActionView},
		"teachers":      {ActionView},
		"subjects":      {ActionView},
		"classes":       {ActionView, ActionCreate, ActionUpdate},
		"attendance":    {ActionView, ActionCreate, ActionUpdate},
		"homework":      {ActionView, ActionCreate, ActionUpdate},
		"exams":         {ActionView, ActionCreate, ActionUpdate},
		"results":       {ActionView, ActionCreate, ActionUpdate},
		"fees":          {ActionView},
		"reports":       {ActionView},
		"notifications": {ActionView},
		"announcements": {ActionView},
		"timetable":     {ActionView, ActionCreate, ActionUpdate},
		"behavior":      {ActionView, ActionCreate, ActionUpdate},
		"leave":         {ActionView, ActionCreate},
		"events":        {ActionView},
		"schedules":     {ActionView, ActionCreate, ActionUpdate, ActionDelete},
	},
	"student": {
		"settings":      {ActionView},
		"subjects":      {ActionView},
		"classes":       {ActionView},
		"attendance":    {ActionView},
		"homework":      {ActionView},
		"exams":         {ActionView},
		"results":       {ActionView},
		"fees":          {ActionView},
		"reports":       {ActionView},
		"notifications": {ActionView},
		"announcements": {ActionView},
		"timetable":     {ActionView},
		"events":        {ActionView},
		"leave":         {ActionView, ActionCreate},
	},
}

// CanAccess returns true when `role` may perform `action` on `feature`.
// `manage` implicitly grants every action.
func CanAccess(role string, feature Feature, action Action) bool {
	allowed, ok := roleAccessMatrix[role][feature]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == action || a == ActionManage {
			return true
		}
	}
	return false
}

// AssertPermission mirrors the Node `assertPermission()` helper. Returns a
// ControlledError(FORBIDDEN, 403) when the requesting user is missing the
// permission. Explicit per-user permissions on the JWT (`feature:action`)
// override the role default.
func AssertPermission(ctx *api.RequestContext, feature Feature, action Action) error {
	wantFA := string(feature) + ":" + string(action)
	wantManage := string(feature) + ":manage"
	for _, p := range ctx.Permissions {
		if p == wantFA || p == wantManage || p == "*" {
			return nil
		}
	}
	if CanAccess(ctx.Role, feature, action) {
		return nil
	}
	return api.NewControlledError(
		"FORBIDDEN",
		"You do not have permission to perform this action.",
		403,
		map[string]string{"feature": string(feature), "action": string(action)},
	)
}
