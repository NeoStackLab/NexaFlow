package service

import (
	"context"
	"errors"
	"testing"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
)

type workflowRepositoryStub struct {
	definition  model.WorkflowDefinition
	transition  model.WorkflowTransition
	defineCalls int
}

func (s *workflowRepositoryStub) Define(_ context.Context, _ string, input model.DefineWorkflowInput) (model.WorkflowDefinition, error) {
	s.defineCalls++
	return model.WorkflowDefinition{Nodes: input.Nodes, Edges: input.Edges}, nil
}
func (*workflowRepositoryStub) List(context.Context, string) ([]model.WorkflowDefinition, error) {
	return nil, nil
}
func (s *workflowRepositoryStub) Get(context.Context, string, string) (model.WorkflowDefinition, error) {
	return s.definition, nil
}
func (*workflowRepositoryStub) Archive(context.Context, string, string, int) error { return nil }
func (s *workflowRepositoryStub) Start(_ context.Context, _ string, _ model.WorkflowDefinition, _, _ string, transition model.WorkflowTransition) (model.WorkflowInstance, error) {
	s.transition = transition
	return model.WorkflowInstance{CurrentNodeID: transition.CurrentNodeID, Status: transition.Status}, nil
}
func (*workflowRepositoryStub) ListInstances(context.Context, string, string) ([]model.WorkflowInstance, error) {
	return nil, nil
}
func (*workflowRepositoryStub) GetInstance(context.Context, string, string) (model.WorkflowInstance, error) {
	return model.WorkflowInstance{}, nil
}
func (*workflowRepositoryStub) ListNotifications(context.Context, string, string) ([]model.Notification, error) {
	return nil, nil
}
func (*workflowRepositoryStub) ReadNotification(context.Context, string, string, string) error {
	return nil
}
func (*workflowRepositoryStub) Advance(context.Context, string, model.WorkflowInstance, string, model.WorkflowActionInput, model.WorkflowTransition) (model.WorkflowInstance, error) {
	return model.WorkflowInstance{}, nil
}

func approvalGraph() ([]model.WorkflowNode, []model.WorkflowEdge) {
	return []model.WorkflowNode{{ID: "start", Type: "start"}, {ID: "approve", Type: "approval", Config: map[string]any{"assignee_role": "admin"}}, {ID: "end", Type: "end"}}, []model.WorkflowEdge{{From: "start", To: "approve"}, {From: "approve", To: "end"}}
}
func TestWorkflowDefinitionRejectsCyclesAndMissingApprovalRole(t *testing.T) {
	nodes, edges := approvalGraph()
	nodes[1].Config = nil
	if err := validateWorkflowGraph(nodes, edges); !errors.Is(err, ErrInvalidWorkflow) {
		t.Fatalf("error=%v", err)
	}
	nodes, edges = approvalGraph()
	edges[1].To = "start"
	if err := validateWorkflowGraph(nodes, edges); !errors.Is(err, ErrInvalidWorkflow) {
		t.Fatalf("cycle error=%v", err)
	}
}
func TestWorkflowTraversalStopsAtApproval(t *testing.T) {
	nodes, edges := approvalGraph()
	transition, err := traverse(model.WorkflowDefinition{Nodes: nodes, Edges: edges}, "", map[string]any{}, "user-1")
	if err != nil || transition.Status != "pending" || transition.CurrentNodeID != "approve" {
		t.Fatalf("transition=%#v error=%v", transition, err)
	}
}
func TestWorkflowConditionAndNotificationComplete(t *testing.T) {
	def := model.WorkflowDefinition{Nodes: []model.WorkflowNode{{ID: "start", Type: "start"}, {ID: "condition", Type: "condition", Config: map[string]any{"field": "amount", "operator": "greater_than", "value": float64(100)}}, {ID: "notify", Type: "notification", Config: map[string]any{"channel": "inapp", "subject": "Large order"}}, {ID: "end", Type: "end"}}, Edges: []model.WorkflowEdge{{From: "start", To: "condition"}, {From: "condition", To: "notify", Condition: "true"}, {From: "condition", To: "end", Condition: "false"}, {From: "notify", To: "end"}}}
	transition, err := traverse(def, "", map[string]any{"amount": float64(150)}, "user-1")
	if err != nil || transition.Status != "completed" || len(transition.Notifications) != 1 {
		t.Fatalf("transition=%#v error=%v", transition, err)
	}
}

func TestWorkflowDefinitionRequiresNotificationRecipientAndCompleteBranches(t *testing.T) {
	nodes := []model.WorkflowNode{{ID: "start", Type: "start"}, {ID: "notify", Type: "notification", Config: map[string]any{"channel": "email"}}, {ID: "end", Type: "end"}}
	edges := []model.WorkflowEdge{{From: "start", To: "notify"}, {From: "notify", To: "end"}}
	if err := validateWorkflowGraph(nodes, edges); !errors.Is(err, ErrInvalidWorkflow) {
		t.Fatalf("notification error=%v", err)
	}
	nodes = []model.WorkflowNode{{ID: "start", Type: "start"}, {ID: "condition", Type: "condition", Config: map[string]any{"field": "amount", "operator": "equals", "value": "1"}}, {ID: "end", Type: "end"}}
	edges = []model.WorkflowEdge{{From: "start", To: "condition"}, {From: "condition", To: "end", Condition: "true"}}
	if err := validateWorkflowGraph(nodes, edges); !errors.Is(err, ErrInvalidWorkflow) {
		t.Fatalf("branch error=%v", err)
	}
}
