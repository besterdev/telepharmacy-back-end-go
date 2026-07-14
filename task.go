package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var validServiceTypes = map[string]bool{"video_call": true, "chat": true, "phone_call": true, "voice_call": true}
var validStatuses = map[string]bool{"new": true, "pending": true, "in_progress": true, "completed": true}

type Task struct {
	ID           uuid.UUID `json:"id"`
	CustomerName string    `json:"customerName"`
	ServiceType  string    `json:"serviceType"`
	Symptom      *string   `json:"symptom"`
	Status       string    `json:"status"`
	CreatedAt    string    `json:"createdAt"`
}

type taskInput struct {
	CustomerName *string `json:"customerName"`
	ServiceType  *string `json:"serviceType"`
	Symptom      *string `json:"symptom"`
	Status       *string `json:"status"`
	CreatedAt    *string `json:"createdAt"`
}

type seedTask struct {
	CustomerName string  `json:"customerName"`
	ServiceType  string  `json:"serviceType"`
	Symptom      *string `json:"symptom"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"createdAt"`
}

type seedFile struct {
	Tasks []seedTask `json:"tasks"`
}

type TaskRepository struct{ pool *pgxpool.Pool }

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository { return &TaskRepository{pool: pool} }

func (r *TaskRepository) List(ctx context.Context) ([]Task, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, customer_name, service_type, symptom, status, created_at FROM tasks ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.CustomerName, &task.ServiceType, &task.Symptom, &task.Status, &task.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Get(ctx context.Context, id uuid.UUID) (Task, error) {
	var task Task
	err := r.pool.QueryRow(ctx, `SELECT id, customer_name, service_type, symptom, status, created_at FROM tasks WHERE id = $1`, id).
		Scan(&task.ID, &task.CustomerName, &task.ServiceType, &task.Symptom, &task.Status, &task.CreatedAt)
	return task, err
}

func (r *TaskRepository) Create(ctx context.Context, task Task) (Task, error) {
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO tasks (id, customer_name, service_type, symptom, status, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, customer_name, service_type, symptom, status, created_at`, task.ID, task.CustomerName, task.ServiceType, task.Symptom, task.Status, task.CreatedAt).
		Scan(&task.ID, &task.CustomerName, &task.ServiceType, &task.Symptom, &task.Status, &task.CreatedAt)
	return task, err
}

func (r *TaskRepository) Replace(ctx context.Context, id uuid.UUID, task Task) (Task, error) {
	err := r.pool.QueryRow(ctx, `UPDATE tasks SET customer_name = $2, service_type = $3, symptom = $4, status = $5, created_at = $6 WHERE id = $1 RETURNING id, customer_name, service_type, symptom, status, created_at`, id, task.CustomerName, task.ServiceType, task.Symptom, task.Status, task.CreatedAt).
		Scan(&task.ID, &task.CustomerName, &task.ServiceType, &task.Symptom, &task.Status, &task.CreatedAt)
	return task, err
}

func (r *TaskRepository) Update(ctx context.Context, id uuid.UUID, input taskInput) (Task, error) {
	current, err := r.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if input.CustomerName != nil {
		current.CustomerName = *input.CustomerName
	}
	if input.ServiceType != nil {
		current.ServiceType = *input.ServiceType
	}
	if input.Symptom != nil {
		current.Symptom = input.Symptom
	}
	if input.Status != nil {
		current.Status = *input.Status
	}
	if input.CreatedAt != nil {
		current.CreatedAt = *input.CreatedAt
	}
	return r.Replace(ctx, id, current)
}

func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *TaskRepository) ReplaceAll(ctx context.Context, tasks []seedTask) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM tasks`); err != nil {
		return err
	}
	for _, item := range tasks {
		if _, err := tx.Exec(ctx, `INSERT INTO tasks (id, customer_name, service_type, symptom, status, created_at) VALUES ($1, $2, $3, $4, $5, $6)`, uuid.New(), item.CustomerName, item.ServiceType, item.Symptom, item.Status, item.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type TaskHandler struct {
	repo         *TaskRepository
	seedFilePath string
}

func NewTaskHandler(repo *TaskRepository, seedFilePath string) *TaskHandler {
	if seedFilePath == "" {
		seedFilePath = "./db.json"
	}
	return &TaskHandler{repo: repo, seedFilePath: seedFilePath}
}

func (h *TaskHandler) List(c *fiber.Ctx) error {
	tasks, err := h.repo.List(c.UserContext())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"data": tasks})
}

func (h *TaskHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	task, err := h.repo.Get(c.UserContext(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"data": task})
}

func (h *TaskHandler) Create(c *fiber.Ctx) error {
	var input taskInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateInput(input, true); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	task := Task{CustomerName: *input.CustomerName, ServiceType: *input.ServiceType, Symptom: input.Symptom, Status: *input.Status, CreatedAt: *input.CreatedAt}
	created, err := h.repo.Create(c.UserContext(), task)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": created})
}

func (h *TaskHandler) Replace(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var input taskInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateInput(input, true); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	task := Task{CustomerName: *input.CustomerName, ServiceType: *input.ServiceType, Symptom: input.Symptom, Status: *input.Status, CreatedAt: *input.CreatedAt}
	updated, err := h.repo.Replace(c.UserContext(), id, task)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"data": updated})
}

func (h *TaskHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var input taskInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateInput(input, false); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	updated, err := h.repo.Update(c.UserContext(), id, input)
	if errors.Is(err, pgx.ErrNoRows) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"data": updated})
}

func (h *TaskHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	if err := h.repo.Delete(c.UserContext(), id); errors.Is(err, pgx.ErrNoRows) {
		return fiber.ErrNotFound
	} else if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *TaskHandler) Seed(c *fiber.Ctx) error {
	contents, err := os.ReadFile(h.seedFilePath)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	var source seedFile
	if err := json.Unmarshal(contents, &source); err != nil {
		return fmt.Errorf("parse seed file: %w", err)
	}
	if len(source.Tasks) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "seed file has no tasks")
	}
	if err := h.repo.ReplaceAll(c.UserContext(), source.Tasks); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "tasks seeded", "count": len(source.Tasks)})
}

func parseID(c *fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, fiber.NewError(fiber.StatusBadRequest, "id must be a valid UUID")
	}
	return id, nil
}

func validateInput(input taskInput, required bool) error {
	if required && (input.CustomerName == nil || input.ServiceType == nil || input.Status == nil || input.CreatedAt == nil) {
		return errors.New("customerName, serviceType, status and createdAt are required")
	}
	if input.ServiceType != nil && !validServiceTypes[*input.ServiceType] {
		return fmt.Errorf("serviceType must be one of: %s", strings.Join(sortedKeys(validServiceTypes), ", "))
	}
	if input.Status != nil && !validStatuses[*input.Status] {
		return fmt.Errorf("status must be one of: %s", strings.Join(sortedKeys(validStatuses), ", "))
	}
	return nil
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
