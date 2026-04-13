package zendesk

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ResultKind string

const (
	ResultKindObject ResultKind = "object"
	ResultKindList   ResultKind = "list"
)

type Result struct {
	Operation string
	Entity    string
	Kind      ResultKind
	Columns   []string
	Object    map[string]any
	Items     []map[string]any
	Page      *PageInfo
}

func (result Result) jsonValue() any {
	switch result.Kind {
	case ResultKindObject:
		return result.Object
	case ResultKindList:
		value := map[string]any{
			"items": result.Items,
		}
		if result.Page != nil {
			value["page"] = result.Page
		}
		return value
	default:
		return nil
	}
}

type QueryEngine struct {
	client *Client
}

type GrepOptions struct {
	Type  string
	Limit int
	Page  int
}

type entitySchema struct {
	DefaultPreset string
	Fields        []string
	Presets       map[string][]string
}

type operationSchema struct {
	Description string
	Entity      string
	Parameters  []map[string]any
	Examples    []string
	Pagination  []string
}

var entitySchemas = map[string]entitySchema{
	"ticket": {
		DefaultPreset: "default",
		Fields:        []string{"id", "subject", "status", "priority", "type", "requester_id", "assignee_id", "organization_id", "updated_at", "created_at", "tags", "attachments", "url"},
		Presets: map[string][]string{
			"minimal":  {"id", "subject", "status", "updated_at"},
			"default":  {"id", "subject", "status", "priority", "requester_id", "assignee_id", "updated_at", "attachments"},
			"overview": {"id", "subject", "status", "priority", "type", "requester_id", "assignee_id", "organization_id", "updated_at", "tags", "attachments"},
		},
	},
	"ticket_comment": {
		DefaultPreset: "default",
		Fields:        []string{"id", "author_id", "public", "created_at", "body", "html_body", "plain_body", "attachments", "via", "metadata", "type"},
		Presets: map[string][]string{
			"minimal":  {"id", "author_id", "public", "created_at", "body"},
			"default":  {"id", "author_id", "public", "created_at", "body", "html_body", "via"},
			"overview": {"id", "author_id", "public", "created_at", "plain_body", "attachments", "via"},
		},
	},
	"user": {
		DefaultPreset: "default",
		Fields:        []string{"id", "name", "email", "role", "organization_id", "active", "suspended", "updated_at", "phone", "tags", "url"},
		Presets: map[string][]string{
			"minimal":  {"id", "name", "role", "organization_id"},
			"default":  {"id", "name", "email", "role", "organization_id", "active", "suspended", "updated_at"},
			"overview": {"id", "name", "email", "role", "organization_id", "phone", "tags", "active", "suspended", "updated_at"},
		},
	},
	"organization": {
		DefaultPreset: "default",
		Fields:        []string{"id", "name", "external_id", "domain_names", "tags", "shared_tickets", "shared_comments", "updated_at", "organization_fields", "url"},
		Presets: map[string][]string{
			"minimal":  {"id", "name"},
			"default":  {"id", "name", "external_id", "shared_tickets", "shared_comments", "updated_at"},
			"overview": {"id", "name", "external_id", "domain_names", "tags", "shared_tickets", "shared_comments", "updated_at"},
		},
	},
	"organization_membership": {
		DefaultPreset: "default",
		Fields:        []string{"id", "user_id", "organization_id", "organization_name", "default", "view_tickets", "updated_at"},
		Presets: map[string][]string{
			"minimal":  {"id", "user_id", "organization_id"},
			"default":  {"id", "user_id", "organization_id", "default", "view_tickets", "updated_at"},
			"overview": {"id", "user_id", "organization_id", "organization_name", "default", "view_tickets", "updated_at"},
		},
	},
	"attachment": {
		DefaultPreset: "default",
		Fields:        []string{"id", "file_name", "size", "content_type", "content_url", "mapped_content_url", "url", "deleted", "inline", "malware_scan_result", "malware_access_override", "ticket_id", "comment_id", "comment_author_id", "comment_public", "comment_created_at", "thumbnails"},
		Presets: map[string][]string{
			"minimal":  {"id", "file_name", "size", "content_type"},
			"default":  {"id", "file_name", "size", "content_type", "content_url", "malware_scan_result"},
			"overview": {"id", "file_name", "size", "content_type", "content_url", "ticket_id", "comment_id", "comment_author_id", "comment_public", "comment_created_at", "malware_scan_result"},
		},
	},
	"search_result": {
		DefaultPreset: "overview",
		Fields:        []string{"result_type", "id", "subject", "name", "status", "priority", "requester_id", "organization_id", "email", "role", "updated_at", "url"},
		Presets: map[string][]string{
			"minimal":  {"result_type", "id", "url"},
			"default":  {"result_type", "id", "url", "updated_at"},
			"overview": {"result_type", "id", "subject", "name", "status", "priority", "requester_id", "organization_id", "email", "role", "updated_at", "url"},
		},
	},
}

var operationSchemas = map[string]operationSchema{
	"schema": {
		Description: "Describe supported query operations, fields, presets, pagination modes, and examples.",
		Examples:    []string{"schema()"},
	},
	"ticket": {
		Description: "Fetch a single ticket by id.",
		Entity:      "ticket",
		Parameters:  []map[string]any{{"name": "id", "type": "string", "optional": false}},
		Examples:    []string{"ticket(12345) { overview }"},
	},
	"tickets": {
		Description: "List tickets with cursor-first pagination.",
		Entity:      "ticket",
		Parameters:  []map[string]any{{"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultCursorLimit}, {"name": "page", "type": "int", "optional": true}, {"name": "per_page", "type": "int", "optional": true}},
		Examples:    []string{"tickets(limit=10) { overview }"},
		Pagination:  []string{"cursor", "offset"},
	},
	"ticket_comments": {
		Description: "List comments for a ticket.",
		Entity:      "ticket_comment",
		Parameters:  []map[string]any{{"name": "ticket_id", "type": "string", "optional": false}, {"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultCursorLimit}},
		Examples:    []string{"ticket_comments(ticket_id=12345, limit=5) { default }"},
		Pagination:  []string{"cursor", "offset"},
	},
	"user": {
		Description: "Fetch a single user by id.",
		Entity:      "user",
		Parameters:  []map[string]any{{"name": "id", "type": "string", "optional": false}},
		Examples:    []string{"user(67890) { default }"},
	},
	"users": {
		Description: "List users with optional role filter.",
		Entity:      "user",
		Parameters:  []map[string]any{{"name": "role", "type": "string", "optional": true}, {"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultCursorLimit}},
		Examples:    []string{"users(limit=10, role=agent) { overview }"},
		Pagination:  []string{"cursor", "offset"},
	},
	"organization": {
		Description: "Fetch a single organization by id.",
		Entity:      "organization",
		Parameters:  []map[string]any{{"name": "id", "type": "string", "optional": false}},
		Examples:    []string{"organization(12) { overview }"},
	},
	"organizations": {
		Description: "List organizations.",
		Entity:      "organization",
		Parameters:  []map[string]any{{"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultCursorLimit}},
		Examples:    []string{"organizations(limit=10) { overview }"},
		Pagination:  []string{"cursor", "offset"},
	},
	"organization_memberships": {
		Description: "List organization memberships for the account, a specific organization, or a specific user.",
		Entity:      "organization_membership",
		Parameters:  []map[string]any{{"name": "organization_id", "type": "string", "optional": true}, {"name": "user_id", "type": "string", "optional": true}, {"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultCursorLimit}},
		Examples:    []string{"organization_memberships(organization_id=12, limit=10) { overview }"},
		Pagination:  []string{"cursor", "offset"},
	},
	"attachment": {
		Description: "Fetch a single attachment by id.",
		Entity:      "attachment",
		Parameters:  []map[string]any{{"name": "id", "type": "string", "optional": false}},
		Examples:    []string{"attachment(498483) { default }"},
	},
	"ticket_attachments": {
		Description: "List attachments from ticket comments, flattened into attachment rows with comment context.",
		Entity:      "attachment",
		Parameters:  []map[string]any{{"name": "ticket_id", "type": "string", "optional": false}, {"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultCursorLimit}},
		Examples:    []string{"ticket_attachments(ticket_id=12345, limit=10) { overview }"},
		Pagination:  []string{"cursor", "offset"},
	},
	"search": {
		Description: "Search across tickets, users, and organizations using Zendesk Search API.",
		Entity:      "search_result",
		Parameters:  []map[string]any{{"name": "query", "type": "string", "optional": false}, {"name": "include", "type": "string", "optional": true}, {"name": "sort_by", "type": "string", "optional": true}, {"name": "sort_order", "type": "string", "optional": true}, {"name": "page", "type": "int", "optional": true}, {"name": "per_page", "type": "int", "optional": true}},
		Examples:    []string{"search(query=\"type:ticket status:open\") { overview }"},
		Pagination:  []string{"offset"},
	},
	"search_count": {
		Description: "Return only the count for a Zendesk search query.",
		Parameters:  []map[string]any{{"name": "query", "type": "string", "optional": false}},
		Examples:    []string{"search_count(query=\"type:ticket status:open\")"},
	},
	"search_export": {
		Description: "Cursor-paginated export search for large result sets.",
		Entity:      "search_result",
		Parameters:  []map[string]any{{"name": "type", "type": "string", "optional": false}, {"name": "query", "type": "string", "optional": false}, {"name": "after", "type": "string", "optional": true}, {"name": "limit", "type": "int", "optional": true, "default": defaultSearchExportSize}},
		Examples:    []string{"search_export(type=ticket, query=\"status:open\", limit=100) { overview }"},
		Pagination:  []string{"cursor"},
	},
}

func NewQueryEngine(client *Client) *QueryEngine {
	return &QueryEngine{client: client}
}

func (e *QueryEngine) Execute(ctx context.Context, raw string) ([]Result, error) {
	if e == nil || e.client == nil {
		return nil, fmt.Errorf("query engine is not configured")
	}

	requests, err := ParseQueryBatch(raw)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(requests))
	for _, request := range requests {
		result, err := e.executeOne(ctx, request)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (e *QueryEngine) Grep(ctx context.Context, text string, opts GrepOptions) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, fmt.Errorf("grep query is empty")
	}

	query := text
	if grepType := strings.TrimSpace(opts.Type); grepType != "" {
		query = "type:" + grepType + " " + query
	}

	response, err := e.client.Search(ctx, SearchOptions{
		Query:   query,
		Page:    opts.Page,
		PerPage: opts.Limit,
	})
	if err != nil {
		return Result{}, err
	}

	items, columns, err := projectList(response.Items, "search_result", []string{"overview"})
	if err != nil {
		return Result{}, err
	}

	return Result{
		Operation: "grep",
		Entity:    "search_result",
		Kind:      ResultKindList,
		Columns:   columns,
		Items:     items,
		Page:      &response.Page,
	}, nil
}

func (e *QueryEngine) executeOne(ctx context.Context, request QueryRequest) (Result, error) {
	switch request.Operation {
	case "schema":
		object, columns := buildSchemaObject()
		return Result{
			Operation: request.Operation,
			Entity:    "schema",
			Kind:      ResultKindObject,
			Columns:   columns,
			Object:    object,
		}, nil
	case "ticket":
		id := request.positionalOr("id")
		if strings.TrimSpace(id) == "" {
			return Result{}, fmt.Errorf("ticket(id) requires an id")
		}
		object, err := e.client.GetTicket(ctx, id)
		if err != nil {
			return Result{}, err
		}
		if selectionNeedsField("ticket", request.Fields, "attachments") {
			attachments, err := e.listTicketAttachmentRefs(ctx, id)
			if err != nil {
				return Result{}, err
			}
			object = cloneMap(object)
			object["attachments"] = attachments
		}
		return newProjectedObjectResult(request.Operation, "ticket", object, request.Fields)
	case "tickets":
		response, err := e.client.ListTickets(ctx, parseListOptions(request))
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "ticket", response, request.Fields)
	case "ticket_comments":
		ticketID := request.param("ticket_id")
		response, err := e.client.ListTicketComments(ctx, TicketCommentsOptions{
			TicketID:    ticketID,
			ListOptions: parseListOptions(request),
		})
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "ticket_comment", response, request.Fields)
	case "user":
		id := request.positionalOr("id")
		if strings.TrimSpace(id) == "" {
			return Result{}, fmt.Errorf("user(id) requires an id")
		}
		object, err := e.client.GetUser(ctx, id)
		if err != nil {
			return Result{}, err
		}
		return newProjectedObjectResult(request.Operation, "user", object, request.Fields)
	case "users":
		response, err := e.client.ListUsers(ctx, UsersOptions{
			ListOptions: parseListOptions(request),
			Role:        request.param("role"),
		})
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "user", response, request.Fields)
	case "organization":
		id := request.positionalOr("id")
		if strings.TrimSpace(id) == "" {
			return Result{}, fmt.Errorf("organization(id) requires an id")
		}
		object, err := e.client.GetOrganization(ctx, id)
		if err != nil {
			return Result{}, err
		}
		return newProjectedObjectResult(request.Operation, "organization", object, request.Fields)
	case "organizations":
		response, err := e.client.ListOrganizations(ctx, parseListOptions(request))
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "organization", response, request.Fields)
	case "organization_memberships":
		response, err := e.client.ListOrganizationMemberships(ctx, OrganizationMembershipOptions{
			OrganizationID: request.param("organization_id"),
			UserID:         request.param("user_id"),
			ListOptions:    parseListOptions(request),
		})
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "organization_membership", response, request.Fields)
	case "attachment":
		id := request.positionalOr("id")
		if strings.TrimSpace(id) == "" {
			return Result{}, fmt.Errorf("attachment(id) requires an id")
		}
		object, err := e.client.GetAttachment(ctx, id)
		if err != nil {
			return Result{}, err
		}
		return newProjectedObjectResult(request.Operation, "attachment", object, request.Fields)
	case "ticket_attachments":
		ticketID := request.param("ticket_id")
		response, err := e.client.ListTicketComments(ctx, TicketCommentsOptions{
			TicketID:    ticketID,
			ListOptions: parseListOptions(request),
		})
		if err != nil {
			return Result{}, err
		}
		items := flattenTicketAttachments(ticketID, response.Items)
		return newProjectedRawListResult(request.Operation, "attachment", items, response.Page, request.Fields)
	case "search":
		response, err := e.client.Search(ctx, SearchOptions{
			Query:     request.param("query"),
			Include:   request.param("include"),
			SortBy:    request.param("sort_by"),
			SortOrder: request.param("sort_order"),
			Page:      request.intParam("page"),
			PerPage:   request.intParam("per_page"),
		})
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "search_result", response, request.Fields)
	case "search_count":
		count, err := e.client.SearchCount(ctx, request.param("query"))
		if err != nil {
			return Result{}, err
		}
		return Result{
			Operation: request.Operation,
			Entity:    "search",
			Kind:      ResultKindObject,
			Columns:   []string{"count"},
			Object:    map[string]any{"count": count},
		}, nil
	case "search_export":
		response, err := e.client.SearchExport(ctx, SearchExportOptions{
			Type:  request.param("type"),
			Query: request.param("query"),
			After: request.param("after"),
			Limit: request.intParam("limit"),
		})
		if err != nil {
			return Result{}, err
		}
		return newProjectedListResult(request.Operation, "search_result", response, request.Fields)
	default:
		return Result{}, unsupportedOperationError(request.Operation)
	}
}

func unsupportedOperationError(operation string) error {
	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "get":
		return fmt.Errorf("unsupported operation %q; use ticket(ID) for one ticket and ticket_comments(ticket_id=ID) for comments", operation)
	case "comments":
		return fmt.Errorf("unsupported operation %q; use ticket_comments(ticket_id=ID)", operation)
	default:
		return fmt.Errorf("unsupported operation %q", operation)
	}
}

func newProjectedObjectResult(operation, entity string, object map[string]any, fields []string) (Result, error) {
	projected, columns, err := projectObject(object, entity, fields)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Operation: operation,
		Entity:    entity,
		Kind:      ResultKindObject,
		Columns:   columns,
		Object:    projected,
	}, nil
}

func newProjectedListResult(operation, entity string, response ListResponse, fields []string) (Result, error) {
	projectedItems, columns, err := projectList(response.Items, entity, fields)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Operation: operation,
		Entity:    entity,
		Kind:      ResultKindList,
		Columns:   columns,
		Items:     projectedItems,
		Page:      &response.Page,
	}, nil
}

func newProjectedRawListResult(operation, entity string, items []map[string]any, page PageInfo, fields []string) (Result, error) {
	projectedItems, columns, err := projectList(items, entity, fields)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Operation: operation,
		Entity:    entity,
		Kind:      ResultKindList,
		Columns:   columns,
		Items:     projectedItems,
		Page:      &page,
	}, nil
}

func projectList(items []map[string]any, entity string, fields []string) ([]map[string]any, []string, error) {
	if len(items) == 0 {
		_, columns, err := projectObject(map[string]any{}, entity, fields)
		if err != nil {
			return nil, nil, err
		}
		return []map[string]any{}, columns, nil
	}

	projectedItems := make([]map[string]any, 0, len(items))
	var columns []string
	for _, item := range items {
		projected, itemColumns, err := projectObject(item, entity, fields)
		if err != nil {
			return nil, nil, err
		}
		if len(columns) == 0 {
			columns = itemColumns
		}
		projectedItems = append(projectedItems, projected)
	}
	return projectedItems, columns, nil
}

func flattenTicketAttachments(ticketID string, comments []map[string]any) []map[string]any {
	var items []map[string]any

	for _, comment := range comments {
		rawAttachments, ok := comment["attachments"].([]any)
		if !ok || len(rawAttachments) == 0 {
			continue
		}

		for _, rawAttachment := range rawAttachments {
			attachment, ok := rawAttachment.(map[string]any)
			if !ok {
				continue
			}

			item := cloneMap(attachment)
			item["ticket_id"] = ticketID
			item["comment_id"] = comment["id"]
			item["comment_author_id"] = comment["author_id"]
			item["comment_public"] = comment["public"]
			item["comment_created_at"] = comment["created_at"]
			items = append(items, item)
		}
	}

	return items
}

func (e *QueryEngine) listTicketAttachmentRefs(ctx context.Context, ticketID string) ([]map[string]any, error) {
	comments, err := e.listAllTicketComments(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	return buildAttachmentRefs(comments), nil
}

func (e *QueryEngine) listAllTicketComments(ctx context.Context, ticketID string) ([]map[string]any, error) {
	var (
		after    string
		comments []map[string]any
	)

	for {
		response, err := e.client.ListTicketComments(ctx, TicketCommentsOptions{
			TicketID: ticketID,
			ListOptions: ListOptions{
				After: after,
				Limit: maxCursorPageSize,
			},
		})
		if err != nil {
			return nil, err
		}
		comments = append(comments, response.Items...)
		if !response.Page.HasMore || strings.TrimSpace(response.Page.AfterCursor) == "" {
			break
		}
		after = response.Page.AfterCursor
	}

	return comments, nil
}

func buildAttachmentRefs(comments []map[string]any) []map[string]any {
	refs := make([]map[string]any, 0)
	seen := map[string]struct{}{}

	for _, comment := range comments {
		rawAttachments, ok := comment["attachments"].([]any)
		if !ok || len(rawAttachments) == 0 {
			continue
		}

		for _, rawAttachment := range rawAttachments {
			attachment, ok := rawAttachment.(map[string]any)
			if !ok {
				continue
			}

			id := attachment["id"]
			fileName := stringValue(attachment["file_name"])
			if fileName == "" {
				fileName = stringValue(attachment["name"])
			}
			if id == nil && fileName == "" {
				continue
			}

			key := fmt.Sprintf("%v|%s", id, fileName)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			refs = append(refs, map[string]any{
				"id":        id,
				"file_name": fileName,
			})
		}
	}

	return refs
}

func projectObject(object map[string]any, entity string, fields []string) (map[string]any, []string, error) {
	schema, ok := entitySchemas[entity]
	if !ok {
		return nil, nil, fmt.Errorf("unknown entity schema %q", entity)
	}

	selection := fields
	if len(selection) == 0 {
		selection = []string{schema.DefaultPreset}
	}

	if len(selection) == 1 {
		preset := selection[0]
		if preset == "full" {
			return cloneMap(object), sortedKeys(object), nil
		}
		if presetFields, ok := schema.Presets[preset]; ok {
			selection = presetFields
		}
	}

	projected := make(map[string]any, len(selection))
	for _, field := range selection {
		projected[field] = object[field]
	}
	return projected, append([]string(nil), selection...), nil
}

func selectionNeedsField(entity string, fields []string, targetField string) bool {
	schema, ok := entitySchemas[entity]
	if !ok {
		return false
	}

	selection := fields
	if len(selection) == 0 {
		selection = []string{schema.DefaultPreset}
	}

	if len(selection) == 1 {
		preset := selection[0]
		if preset == "full" {
			return true
		}
		if presetFields, ok := schema.Presets[preset]; ok {
			selection = presetFields
		}
	}

	for _, field := range selection {
		if field == targetField {
			return true
		}
	}
	return false
}

func parseListOptions(request QueryRequest) ListOptions {
	return ListOptions{
		After:   request.param("after"),
		Limit:   request.intParam("limit"),
		Page:    request.intParam("page"),
		PerPage: request.intParam("per_page"),
	}
}

func (request QueryRequest) positionalOr(key string) string {
	if strings.TrimSpace(request.Positional) != "" {
		return request.Positional
	}
	return request.param(key)
}

func (request QueryRequest) param(key string) string {
	if request.Params == nil {
		return ""
	}
	return request.Params[key]
}

func (request QueryRequest) intParam(key string) int {
	value := strings.TrimSpace(request.param(key))
	if value == "" {
		return 0
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return number
}

func buildSchemaObject() (map[string]any, []string) {
	operations := make([]string, 0, len(operationSchemas))
	operationMetadata := map[string]any{}
	pagination := map[string]any{}
	fields := map[string]any{}
	presets := map[string]any{}
	defaultFields := map[string]any{}

	for name, schema := range operationSchemas {
		operations = append(operations, name)
		operationMetadata[name] = map[string]any{
			"description": schema.Description,
			"entity":      schema.Entity,
			"parameters":  schema.Parameters,
			"examples":    schema.Examples,
		}
		if len(schema.Pagination) > 0 {
			pagination[name] = schema.Pagination
		}
	}
	sort.Strings(operations)

	entityNames := make([]string, 0, len(entitySchemas))
	for entityName, schema := range entitySchemas {
		entityNames = append(entityNames, entityName)
		fields[entityName] = schema.Fields
		presets[entityName] = schema.Presets
		defaultFields[entityName] = []string{schema.DefaultPreset}
	}
	sort.Strings(entityNames)

	return map[string]any{
		"operations":        operations,
		"entities":          entityNames,
		"formats":           []string{"json", "compact"},
		"pagination":        pagination,
		"fields":            fields,
		"presets":           presets,
		"default_fields":    defaultFields,
		"operationMetadata": operationMetadata,
	}, []string{"operations", "entities", "formats", "pagination", "fields", "presets", "default_fields", "operationMetadata"}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func sortedKeys(input map[string]any) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
