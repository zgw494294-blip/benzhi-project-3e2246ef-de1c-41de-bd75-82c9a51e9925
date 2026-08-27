package application

import "benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"

var commandRoles = map[string]map[string]bool{
	"create_case":            {"archivist": true},
	"update_case_metadata":   {"archivist": true},
	"add_recording":          {"archivist": true},
	"add_consent":            {"archivist": true},
	"add_segment":            {"archivist": true},
	"batch_add_segments":     {"archivist": true},
	"lock_intake":            {"archivist": true},
	"set_marks":              {"archivist": true},
	"generate_redaction":     {"archivist": true},
	"submit_review":          {"archivist": true},
	"resolve_review_finding": {"archivist": true},
	"decide_review":          {"reviewer": true},
	"release":                {"manager": true},
}

func authorize(command string, actor Actor) error {
	if actor.ID == "" || actor.Role == "" {
		return domain.Forbidden("actor_required", "必须提供操作者身份和角色")
	}
	roles, known := commandRoles[command]
	if !known {
		return domain.Validation("unknown_command", "未知命令 %s", command)
	}
	if !roles[actor.Role] {
		return domain.Forbidden("role_forbidden", "角色无权执行该命令")
	}
	return nil
}
