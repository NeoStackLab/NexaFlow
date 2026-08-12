package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

var (
	ErrInvalidWorkflow      = errors.New("invalid workflow")
	ErrWorkflowActionDenied = errors.New("workflow action denied")
)

type WorkflowRepository interface {
	Define(context.Context, string, model.DefineWorkflowInput) (model.WorkflowDefinition, error)
	List(context.Context, string) ([]model.WorkflowDefinition, error)
	Get(context.Context, string, string) (model.WorkflowDefinition, error)
	Archive(context.Context, string, string, int) error
	Start(context.Context, string, model.WorkflowDefinition, string, string, model.WorkflowTransition) (model.WorkflowInstance, error)
	ListInstances(context.Context, string, string) ([]model.WorkflowInstance, error)
	GetInstance(context.Context, string, string) (model.WorkflowInstance, error)
	Advance(context.Context, string, model.WorkflowInstance, string, model.WorkflowActionInput, model.WorkflowTransition) (model.WorkflowInstance, error)
	ListNotifications(context.Context, string, string) ([]model.Notification, error)
	ReadNotification(context.Context, string, string, string) error
}

type WorkflowService interface {
	Define(context.Context, string, model.DefineWorkflowInput) (model.WorkflowDefinition, error)
	List(context.Context, string) ([]model.WorkflowDefinition, error)
	Get(context.Context, string, string) (model.WorkflowDefinition, error)
	Archive(context.Context, string, string, int) error
	Start(context.Context, string, string, string, string) (model.WorkflowInstance, error)
	ListInstances(context.Context, string, string) ([]model.WorkflowInstance, error)
	Act(context.Context, string, string, string, []string, model.WorkflowActionInput) (model.WorkflowInstance, error)
	ListNotifications(context.Context, string, string) ([]model.Notification, error)
	ReadNotification(context.Context, string, string, string) error
}

type workflowService struct {
	repository WorkflowRepository
	models     DynamicModelService
	records    DynamicRecordService
}

func NewWorkflowService(repository WorkflowRepository, models DynamicModelService, records DynamicRecordService) WorkflowService {
	return &workflowService{repository: repository, models: models, records: records}
}

func (s *workflowService) Define(ctx context.Context, tenantID string, input model.DefineWorkflowInput) (model.WorkflowDefinition, error) {
	input.Name, input.Slug, input.Description = strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Slug)), strings.TrimSpace(input.Description)
	if len(input.Name) < 2 || len(input.Name) > 120 || !entitySlugPattern.MatchString(input.Slug) || len(input.Description) > 500 {
		return model.WorkflowDefinition{}, fmt.Errorf("%w: invalid name, slug, or description", ErrInvalidWorkflow)
	}
	if input.ID != "" && input.ExpectedVersion < 1 {
		return model.WorkflowDefinition{}, fmt.Errorf("%w: expected_version is required", ErrInvalidWorkflow)
	}
	if input.ID != "" {
		existing, err := s.repository.Get(ctx, tenantID, input.ID)
		if err != nil {
			return model.WorkflowDefinition{}, err
		}
		if existing.EntityID != input.EntityID {
			return model.WorkflowDefinition{}, fmt.Errorf("%w: a saved workflow cannot change entity", ErrInvalidWorkflow)
		}
	}
	if _, err := s.models.Get(ctx, tenantID, input.EntityID); err != nil {
		return model.WorkflowDefinition{}, err
	}
	if err := validateWorkflowGraph(input.Nodes, input.Edges); err != nil {
		return model.WorkflowDefinition{}, err
	}
	return s.repository.Define(ctx, tenantID, input)
}
func (s *workflowService) List(ctx context.Context, tenantID string) ([]model.WorkflowDefinition, error) {
	return s.repository.List(ctx, tenantID)
}
func (s *workflowService) Get(ctx context.Context, tenantID, id string) (model.WorkflowDefinition, error) {
	return s.repository.Get(ctx, tenantID, id)
}
func (s *workflowService) Archive(ctx context.Context, tenantID, id string, version int) error {
	if version < 1 {
		return fmt.Errorf("%w: expected_version is required", ErrInvalidWorkflow)
	}
	return s.repository.Archive(ctx, tenantID, id, version)
}
func (s *workflowService) ListInstances(ctx context.Context, tenantID, workflowID string) ([]model.WorkflowInstance, error) {
	return s.repository.ListInstances(ctx, tenantID, workflowID)
}
func (s *workflowService) ListNotifications(ctx context.Context, tenantID, userID string) ([]model.Notification, error) {
	return s.repository.ListNotifications(ctx, tenantID, userID)
}
func (s *workflowService) ReadNotification(ctx context.Context, tenantID, userID, notificationID string) error {
	return s.repository.ReadNotification(ctx, tenantID, userID, notificationID)
}

func (s *workflowService) Start(ctx context.Context, tenantID, workflowID, recordID, actorID string) (model.WorkflowInstance, error) {
	definition, err := s.repository.Get(ctx, tenantID, workflowID)
	if err != nil {
		return model.WorkflowInstance{}, err
	}
	record, err := s.records.Get(ctx, tenantID, definition.EntityID, recordID)
	if err != nil {
		return model.WorkflowInstance{}, err
	}
	transition, err := traverse(definition, "", record.Values, actorID)
	if err != nil {
		return model.WorkflowInstance{}, err
	}
	return s.repository.Start(ctx, tenantID, definition, recordID, actorID, transition)
}
func (s *workflowService) Act(ctx context.Context, tenantID, instanceID, actorID string, roles []string, input model.WorkflowActionInput) (model.WorkflowInstance, error) {
	if input.ExpectedVersion < 1 || (input.Action != "approve" && input.Action != "reject") {
		return model.WorkflowInstance{}, fmt.Errorf("%w: invalid action or expected_version", ErrInvalidWorkflow)
	}
	instance, err := s.repository.GetInstance(ctx, tenantID, instanceID)
	if err != nil {
		return model.WorkflowInstance{}, err
	}
	if instance.Status != "pending" || instance.Version != input.ExpectedVersion {
		return model.WorkflowInstance{}, fmt.Errorf("%w: instance is not actionable", ErrWorkflowActionDenied)
	}
	definition, err := s.repository.Get(ctx, tenantID, instance.WorkflowID)
	if err != nil {
		return model.WorkflowInstance{}, err
	}
	node := findNode(definition.Nodes, instance.CurrentNodeID)
	if node == nil || node.Type != "approval" {
		return model.WorkflowInstance{}, fmt.Errorf("%w: current node is not approval", ErrWorkflowActionDenied)
	}
	requiredRole, _ := node.Config["assignee_role"].(string)
	if requiredRole != "" && !hasRole(roles, requiredRole) && !hasRole(roles, "super_admin") {
		return model.WorkflowInstance{}, fmt.Errorf("%w: role %q is required", ErrWorkflowActionDenied, requiredRole)
	}
	transition := model.WorkflowTransition{CurrentNodeID: node.ID, Status: "rejected"}
	if input.Action == "approve" {
		record, err := s.records.Get(ctx, tenantID, instance.EntityID, instance.RecordID)
		if err != nil {
			return model.WorkflowInstance{}, err
		}
		transition, err = traverse(definition, node.ID, record.Values, instance.SubmittedBy)
		if err != nil {
			return model.WorkflowInstance{}, err
		}
	}
	return s.repository.Advance(ctx, tenantID, instance, actorID, input, transition)
}

func validateWorkflowGraph(nodes []model.WorkflowNode, edges []model.WorkflowEdge) error {
	if len(nodes) < 2 || len(nodes) > 100 {
		return fmt.Errorf("%w: workflow must contain 2-100 nodes", ErrInvalidWorkflow)
	}
	byID := map[string]model.WorkflowNode{}
	starts, ends := 0, 0
	validTypes := map[string]bool{"start": true, "approval": true, "condition": true, "notification": true, "end": true}
	for _, node := range nodes {
		if node.ID == "" || byID[node.ID].ID != "" || !validTypes[node.Type] {
			return fmt.Errorf("%w: invalid or duplicate node", ErrInvalidWorkflow)
		}
		byID[node.ID] = node
		if node.Type == "start" {
			starts++
		}
		if node.Type == "end" {
			ends++
		}
		if node.Type == "approval" {
			if role, _ := node.Config["assignee_role"].(string); role == "" {
				return fmt.Errorf("%w: approval node requires assignee_role", ErrInvalidWorkflow)
			}
		}
		if node.Type == "notification" {
			channel, _ := node.Config["channel"].(string)
			if channel != "inapp" && channel != "email" && channel != "webhook" {
				return fmt.Errorf("%w: invalid notification channel", ErrInvalidWorkflow)
			}
			if recipient, _ := node.Config["recipient"].(string); channel != "inapp" && strings.TrimSpace(recipient) == "" {
				return fmt.Errorf("%w: %s notification requires recipient", ErrInvalidWorkflow, channel)
			}
		}
	}
	if starts != 1 || ends < 1 {
		return fmt.Errorf("%w: exactly one start and at least one end are required", ErrInvalidWorkflow)
	}
	out := map[string][]model.WorkflowEdge{}
	for _, edge := range edges {
		if byID[edge.From].ID == "" || byID[edge.To].ID == "" || edge.From == edge.To {
			return fmt.Errorf("%w: invalid edge", ErrInvalidWorkflow)
		}
		out[edge.From] = append(out[edge.From], edge)
	}
	for _, node := range nodes {
		if node.Type != "condition" {
			continue
		}
		branches := map[string]int{}
		for _, edge := range out[node.ID] {
			branches[edge.Condition]++
		}
		if branches["true"] != 1 || branches["false"] != 1 || len(out[node.ID]) != 2 {
			return fmt.Errorf("%w: condition node requires true and false branches", ErrInvalidWorkflow)
		}
	}
	var start string
	for _, node := range nodes {
		if node.Type == "start" {
			start = node.ID
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("%w: cycles are not supported", ErrInvalidWorkflow)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		if byID[id].Type != "end" && len(out[id]) == 0 {
			return fmt.Errorf("%w: node %q has no outgoing edge", ErrInvalidWorkflow, id)
		}
		for _, edge := range out[id] {
			if err := walk(edge.To); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	if err := walk(start); err != nil {
		return err
	}
	if len(visited) != len(nodes) {
		return fmt.Errorf("%w: every node must be reachable", ErrInvalidWorkflow)
	}
	return nil
}

func traverse(def model.WorkflowDefinition, after string, values map[string]any, actorID string) (model.WorkflowTransition, error) {
	current := after
	notifications := []model.Notification{}
	if current == "" {
		for _, node := range def.Nodes {
			if node.Type == "start" {
				current = node.ID
				break
			}
		}
	}
	for steps := 0; steps <= len(def.Nodes); steps++ {
		node := findNode(def.Nodes, current)
		if node == nil {
			return model.WorkflowTransition{}, fmt.Errorf("%w: node not found", ErrInvalidWorkflow)
		}
		if node.Type == "approval" && current != after {
			return model.WorkflowTransition{CurrentNodeID: current, Status: "pending", Notifications: notifications}, nil
		}
		if node.Type == "end" {
			return model.WorkflowTransition{CurrentNodeID: current, Status: "completed", Notifications: notifications}, nil
		}
		if node.Type == "notification" {
			notifications = append(notifications, buildNotification(*node, actorID))
		}
		edge, ok := chooseEdge(*node, def.Edges, values)
		if !ok {
			return model.WorkflowTransition{}, fmt.Errorf("%w: no matching path from %q", ErrInvalidWorkflow, current)
		}
		current = edge.To
	}
	return model.WorkflowTransition{}, fmt.Errorf("%w: traversal limit exceeded", ErrInvalidWorkflow)
}

func chooseEdge(node model.WorkflowNode, edges []model.WorkflowEdge, values map[string]any) (model.WorkflowEdge, bool) {
	candidates := []model.WorkflowEdge{}
	for _, edge := range edges {
		if edge.From == node.ID {
			candidates = append(candidates, edge)
		}
	}
	if node.Type != "condition" {
		if len(candidates) == 1 {
			return candidates[0], true
		}
		return model.WorkflowEdge{}, false
	}
	field, _ := node.Config["field"].(string)
	operator, _ := node.Config["operator"].(string)
	expected := node.Config["value"]
	matched := compare(values[field], operator, expected)
	wanted := "false"
	if matched {
		wanted = "true"
	}
	for _, edge := range candidates {
		if edge.Condition == wanted {
			return edge, true
		}
	}
	return model.WorkflowEdge{}, false
}
func compare(actual any, operator string, expected any) bool {
	switch operator {
	case "equals":
		return fmt.Sprint(actual) == fmt.Sprint(expected)
	case "not_equals":
		return fmt.Sprint(actual) != fmt.Sprint(expected)
	case "greater_than":
		a, aerr := strconv.ParseFloat(fmt.Sprint(actual), 64)
		b, berr := strconv.ParseFloat(fmt.Sprint(expected), 64)
		return aerr == nil && berr == nil && a > b
	case "contains":
		return strings.Contains(fmt.Sprint(actual), fmt.Sprint(expected))
	default:
		return false
	}
}
func findNode(nodes []model.WorkflowNode, id string) *model.WorkflowNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}
func hasRole(roles []string, wanted string) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}
func buildNotification(node model.WorkflowNode, actorID string) model.Notification {
	channel, _ := node.Config["channel"].(string)
	recipient, _ := node.Config["recipient"].(string)
	subject, _ := node.Config["subject"].(string)
	body, _ := node.Config["body"].(string)
	return model.Notification{UserID: actorID, Channel: channel, Recipient: recipient, Subject: subject, Body: body, Status: "pending"}
}
