package synth

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	"github.com/movebigrocks/platform/pkg/id"
)

// Generator creates synthetic test data
type Generator struct {
	store  stores.Store
	rand   *rand.Rand
	config *Config
}

// Config holds configuration for data generation
type Config struct {
	// Counts
	NumWorkspaces      int
	NumAdminsPerWs     int
	NumAgentsPerWs     int
	NumCustomersPerWs  int
	NumCasesPerWs      int
	NumMessagesPerCase int

	// Timing
	CaseSpreadDays int // How many days back to spread case creation

	// Options
	Verbose bool
	DryRun  bool
}

// DefaultConfig returns sensible defaults for testing
func DefaultConfig() *Config {
	return &Config{
		NumWorkspaces:      2,
		NumAdminsPerWs:     1,
		NumAgentsPerWs:     3,
		NumCustomersPerWs:  10,
		NumCasesPerWs:      20,
		NumMessagesPerCase: 4,
		CaseSpreadDays:     30,
		Verbose:            true,
		DryRun:             false,
	}
}

// NewGenerator creates a new synthetic data generator
func NewGenerator(store stores.Store, config *Config) *Generator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Generator{
		store:  store,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())),
		config: config,
	}
}

// GeneratedData holds all generated entities
type GeneratedData struct {
	SuperAdmin  *platformdomain.User
	Workspaces  []*WorkspaceData
	StartTime   time.Time
	EndTime     time.Time
	TotalCases  int
	TotalEmails int
}

// WorkspaceData holds workspace-specific generated data
type WorkspaceData struct {
	Workspace *platformdomain.Workspace
	Settings  *platformdomain.WorkspaceSettings
	Admins    []*platformdomain.User
	Agents    []*platformdomain.User
	Customers []*Contact
	Teams     []*Team
	Cases     []*CaseData
}

// Contact represents a customer contact
type Contact struct {
	ID        string
	Name      string
	Email     string
	Company   string
	CreatedAt time.Time
}

// Team represents a support team
type Team struct {
	ID        string
	Name      string
	Slug      string
	Members   []string // User IDs
	CreatedAt time.Time
}

// CaseData holds case-specific generated data
type CaseData struct {
	Case     *servicedomain.Case
	Emails   []*servicedomain.InboundEmail
	Replies  []*servicedomain.OutboundEmail
	Timeline []TimelineEvent
}

// TimelineEvent represents an event in case timeline
type TimelineEvent struct {
	Type      string
	Timestamp time.Time
	ActorID   string
	ActorName string
	Details   string
}

// Generate creates all synthetic data
func (g *Generator) Generate(ctx context.Context) (*GeneratedData, error) {
	g.log("Starting synthetic data generation...")
	startTime := time.Now()

	data := &GeneratedData{
		StartTime:  startTime,
		Workspaces: make([]*WorkspaceData, 0, g.config.NumWorkspaces),
	}

	// Create super admin
	superAdmin, err := g.createSuperAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create super admin: %w", err)
	}
	data.SuperAdmin = superAdmin
	g.log("Created super admin: %s (%s)", superAdmin.Name, superAdmin.Email)

	// Create workspaces with all data
	for i := 0; i < g.config.NumWorkspaces; i++ {
		wsData, err := g.generateWorkspaceData(ctx, i)
		if err != nil {
			return nil, fmt.Errorf("failed to generate workspace %d: %w", i, err)
		}
		data.Workspaces = append(data.Workspaces, wsData)
		data.TotalCases += len(wsData.Cases)
		for _, c := range wsData.Cases {
			data.TotalEmails += len(c.Emails) + len(c.Replies)
		}
		g.log("Generated workspace '%s' with %d cases", wsData.Workspace.Name, len(wsData.Cases))
	}

	data.EndTime = time.Now()
	g.log("Generation complete in %v", data.EndTime.Sub(data.StartTime))
	g.log("Total: %d workspaces, %d cases, %d emails", len(data.Workspaces), data.TotalCases, data.TotalEmails)

	return data, nil
}

// generateWorkspaceData creates a complete workspace with all related data
func (g *Generator) generateWorkspaceData(ctx context.Context, index int) (*WorkspaceData, error) {
	wsData := &WorkspaceData{
		Admins:    make([]*platformdomain.User, 0, g.config.NumAdminsPerWs),
		Agents:    make([]*platformdomain.User, 0, g.config.NumAgentsPerWs),
		Customers: make([]*Contact, 0, g.config.NumCustomersPerWs),
		Teams:     make([]*Team, 0),
		Cases:     make([]*CaseData, 0, g.config.NumCasesPerWs),
	}

	// Create workspace
	ws, settings, err := g.createWorkspace(ctx, index)
	if err != nil {
		return nil, err
	}
	wsData.Workspace = ws
	wsData.Settings = settings

	// Create teams
	teams, err := g.createTeams(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	wsData.Teams = teams

	// Create admin users
	for i := 0; i < g.config.NumAdminsPerWs; i++ {
		admin, err := g.createWorkspaceAdmin(ctx, ws.ID, i)
		if err != nil {
			return nil, err
		}
		wsData.Admins = append(wsData.Admins, admin)
	}

	// Create agent users
	for i := 0; i < g.config.NumAgentsPerWs; i++ {
		agent, err := g.createAgent(ctx, ws.ID, i, teams)
		if err != nil {
			return nil, err
		}
		wsData.Agents = append(wsData.Agents, agent)
	}

	// Create customer contacts
	for i := 0; i < g.config.NumCustomersPerWs; i++ {
		customer := g.generateCustomer(i)
		wsData.Customers = append(wsData.Customers, customer)
	}

	// Create cases with conversations
	allAgents := append(append([]*platformdomain.User{}, wsData.Admins...), wsData.Agents...)
	for i := 0; i < g.config.NumCasesPerWs; i++ {
		caseData, err := g.generateCase(ctx, ws.ID, wsData.Customers, allAgents, i)
		if err != nil {
			return nil, err
		}
		wsData.Cases = append(wsData.Cases, caseData)
	}

	return wsData, nil
}

// createSuperAdmin creates the instance super admin
func (g *Generator) createSuperAdmin(ctx context.Context) (*platformdomain.User, error) {
	superAdminRole := platformdomain.InstanceRoleSuperAdmin
	user := &platformdomain.User{
		ID:            id.New(),
		Email:         "admin@mbr.local",
		Name:          "System Administrator",
		InstanceRole:  &superAdminRole,
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if !g.config.DryRun {
		if err := g.store.Users().CreateUser(ctx, user); err != nil {
			return nil, err
		}
	}

	return user, nil
}

// createWorkspace creates a workspace with settings
func (g *Generator) createWorkspace(ctx context.Context, index int) (*platformdomain.Workspace, *platformdomain.WorkspaceSettings, error) {
	wsName := workspaceNames[index%len(workspaceNames)]
	ws := &platformdomain.Workspace{
		ID:          id.New(),
		Name:        wsName.Name,
		Slug:        wsName.Slug,
		Description: wsName.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	settings := &platformdomain.WorkspaceSettings{
		ID:                  id.New(),
		WorkspaceID:         ws.ID,
		WorkspaceName:       ws.Name,
		DefaultCasePriority: servicedomain.CasePriorityMedium,
		AutoAssignCases:     true,
		Timezone:            "America/New_York",
		DefaultSLAHours:     4,
		SLAByPriority: map[string]int{
			string(servicedomain.CasePriorityUrgent): 1,
			string(servicedomain.CasePriorityHigh):   4,
			string(servicedomain.CasePriorityMedium): 8,
			string(servicedomain.CasePriorityLow):    24,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if !g.config.DryRun {
		if err := g.store.Workspaces().CreateWorkspace(ctx, ws); err != nil {
			return nil, nil, err
		}
		if err := g.store.Workspaces().CreateWorkspaceSettings(ctx, settings); err != nil {
			return nil, nil, err
		}
	}

	return ws, settings, nil
}

// createTeams creates support teams for a workspace
func (g *Generator) createTeams(ctx context.Context, workspaceID string) ([]*Team, error) {
	teams := make([]*Team, 0, len(teamTemplates))

	for _, tmpl := range teamTemplates {
		team := &Team{
			ID:        id.New(),
			Name:      tmpl.Name,
			Slug:      tmpl.Slug,
			Members:   []string{},
			CreatedAt: time.Now(),
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// createWorkspaceAdmin creates an admin for a workspace
func (g *Generator) createWorkspaceAdmin(ctx context.Context, workspaceID string, index int) (*platformdomain.User, error) {
	person := g.randomPerson()
	user := &platformdomain.User{
		ID:            id.New(),
		Email:         fmt.Sprintf("admin%d@%s.mbr.local", index+1, workspaceID[:8]),
		Name:          person.FullName(),
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if !g.config.DryRun {
		if err := g.store.Users().CreateUser(ctx, user); err != nil {
			return nil, err
		}

		// Create workspace role
		role := &platformdomain.UserWorkspaceRole{
			ID:          id.New(),
			UserID:      user.ID,
			WorkspaceID: workspaceID,
			Role:        platformdomain.WorkspaceRoleAdmin,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := g.store.Workspaces().CreateUserWorkspaceRole(ctx, role); err != nil {
			return nil, err
		}
	}

	return user, nil
}

// createAgent creates a support agent for a workspace
func (g *Generator) createAgent(ctx context.Context, workspaceID string, index int, teams []*Team) (*platformdomain.User, error) {
	person := g.randomPerson()
	user := &platformdomain.User{
		ID:            id.New(),
		Email:         fmt.Sprintf("agent%d@%s.mbr.local", index+1, workspaceID[:8]),
		Name:          person.FullName(),
		IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if !g.config.DryRun {
		if err := g.store.Users().CreateUser(ctx, user); err != nil {
			return nil, err
		}

		// Create workspace role (agents are members)
		role := &platformdomain.UserWorkspaceRole{
			ID:          id.New(),
			UserID:      user.ID,
			WorkspaceID: workspaceID,
			Role:        platformdomain.WorkspaceRoleMember,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := g.store.Workspaces().CreateUserWorkspaceRole(ctx, role); err != nil {
			return nil, err
		}
	}

	// Assign to a team
	if len(teams) > 0 {
		team := teams[g.rand.Intn(len(teams))]
		team.Members = append(team.Members, user.ID)
	}

	return user, nil
}

// generateCustomer creates a customer contact
func (g *Generator) generateCustomer(index int) *Contact {
	person := g.randomPerson()
	company := companies[g.rand.Intn(len(companies))]

	return &Contact{
		ID:        id.New(),
		Name:      person.FullName(),
		Email:     fmt.Sprintf("%s.%s@%s", person.FirstName, person.LastName, company.Domain),
		Company:   company.Name,
		CreatedAt: time.Now(),
	}
}

// generateCase creates a case with full conversation history
func (g *Generator) generateCase(ctx context.Context, workspaceID string, customers []*Contact, agents []*platformdomain.User, index int) (*CaseData, error) {
	// Pick random customer and agent
	customer := customers[g.rand.Intn(len(customers))]
	assignedAgent := agents[g.rand.Intn(len(agents))]

	// Pick a case scenario
	scenario := caseScenarios[g.rand.Intn(len(caseScenarios))]

	// Random creation time within spread
	daysAgo := g.rand.Intn(g.config.CaseSpreadDays)
	createdAt := time.Now().AddDate(0, 0, -daysAgo)

	// Determine final status based on age
	status := g.determineCaseStatus(daysAgo)
	priority := scenario.Priority

	// Create case using constructor
	caseObj := servicedomain.NewCase(workspaceID, scenario.Subject, customer.Email)
	caseObj.ID = id.New()
	caseObj.GenerateHumanID("test") // Generate test human ID
	caseObj.Description = scenario.InitialMessage
	caseObj.Status = status
	caseObj.Priority = priority
	caseObj.Channel = servicedomain.CaseChannelEmail
	caseObj.ContactID = customer.ID
	caseObj.ContactName = customer.Name
	caseObj.AssignedToID = assignedAgent.ID
	caseObj.Tags = scenario.Tags
	caseObj.CreatedAt = createdAt
	caseObj.UpdatedAt = createdAt

	caseData := &CaseData{
		Case:     caseObj,
		Emails:   make([]*servicedomain.InboundEmail, 0),
		Replies:  make([]*servicedomain.OutboundEmail, 0),
		Timeline: make([]TimelineEvent, 0),
	}

	// Generate conversation
	g.generateConversation(caseData, customer, assignedAgent, scenario, createdAt)

	// Save case if not dry run
	if !g.config.DryRun {
		if err := g.store.Cases().CreateCase(ctx, caseObj); err != nil {
			return nil, err
		}

		// Save emails
		for _, email := range caseData.Emails {
			if err := g.store.InboundEmails().CreateInboundEmail(ctx, email); err != nil {
				return nil, err
			}
		}
		for _, reply := range caseData.Replies {
			if err := g.store.OutboundEmails().CreateOutboundEmail(ctx, reply); err != nil {
				return nil, err
			}
		}
	}

	return caseData, nil
}

// generateConversation creates the email thread for a case
func (g *Generator) generateConversation(caseData *CaseData, customer *Contact, agent *platformdomain.User, scenario *CaseScenario, startTime time.Time) {
	currentTime := startTime

	// Initial customer email
	initialEmail := &servicedomain.InboundEmail{
		ID:          id.New(),
		WorkspaceID: caseData.Case.WorkspaceID,
		CaseID:      caseData.Case.ID,
		MessageID:   fmt.Sprintf("<%s@mail.example.com>", id.New()[:16]),
		FromEmail:   customer.Email,
		FromName:    customer.Name,
		ToEmails:    []string{"support@mbr.local"},
		Subject:     caseData.Case.Subject,
		TextContent: scenario.InitialMessage,
		HTMLContent: fmt.Sprintf("<p>%s</p>", scenario.InitialMessage),
		ReceivedAt:  currentTime,
		ProcessedAt: &currentTime,
		CreatedAt:   currentTime,
	}
	caseData.Emails = append(caseData.Emails, initialEmail)
	caseData.Timeline = append(caseData.Timeline, TimelineEvent{
		Type:      "email_received",
		Timestamp: currentTime,
		ActorID:   customer.ID,
		ActorName: customer.Name,
		Details:   "Initial email received",
	})

	// Generate back-and-forth messages
	numExchanges := g.rand.Intn(g.config.NumMessagesPerCase) + 1
	for i := 0; i < numExchanges; i++ {
		// Agent response (2-8 hours later)
		currentTime = currentTime.Add(time.Duration(2+g.rand.Intn(6)) * time.Hour)

		agentReply := g.generateAgentReply(caseData.Case, agent, customer, scenario, i, currentTime)
		caseData.Replies = append(caseData.Replies, agentReply)
		caseData.Timeline = append(caseData.Timeline, TimelineEvent{
			Type:      "reply_sent",
			Timestamp: currentTime,
			ActorID:   agent.ID,
			ActorName: agent.Name,
			Details:   "Agent replied",
		})

		// Customer follow-up (4-24 hours later, 60% chance)
		if g.rand.Float32() < 0.6 && i < numExchanges-1 {
			currentTime = currentTime.Add(time.Duration(4+g.rand.Intn(20)) * time.Hour)

			followUp := g.generateCustomerFollowUp(caseData.Case, customer, scenario, i, currentTime)
			caseData.Emails = append(caseData.Emails, followUp)
			caseData.Timeline = append(caseData.Timeline, TimelineEvent{
				Type:      "email_received",
				Timestamp: currentTime,
				ActorID:   customer.ID,
				ActorName: customer.Name,
				Details:   "Customer follow-up",
			})
		}
	}

	// Update case timestamps
	caseData.Case.UpdatedAt = currentTime
	if caseData.Case.Status == servicedomain.CaseStatusResolved || caseData.Case.Status == servicedomain.CaseStatusClosed {
		caseData.Case.ClosedAt = &currentTime
	}
}

// generateAgentReply creates an agent's email response
func (g *Generator) generateAgentReply(caseObj *servicedomain.Case, agent *platformdomain.User, customer *Contact, scenario *CaseScenario, exchangeNum int, timestamp time.Time) *servicedomain.OutboundEmail {
	// Pick appropriate response template
	response := scenario.AgentResponses[exchangeNum%len(scenario.AgentResponses)]

	return &servicedomain.OutboundEmail{
		ID:                id.New(),
		WorkspaceID:       caseObj.WorkspaceID,
		CaseID:            caseObj.ID,
		ProviderMessageID: fmt.Sprintf("<%s@mbr.local>", id.New()[:16]),
		FromEmail:         "support@mbr.local",
		FromName:          agent.Name,
		ToEmails:          []string{customer.Email},
		Subject:           fmt.Sprintf("Re: %s", caseObj.Subject),
		TextContent:       response,
		HTMLContent:       fmt.Sprintf("<p>Hi %s,</p><p>%s</p><p>Best regards,<br>%s</p>", customer.Name, response, agent.Name),
		Status:            servicedomain.EmailStatusSent,
		SentAt:            &timestamp,
		CreatedAt:         timestamp,
		UpdatedAt:         timestamp,
	}
}

// generateCustomerFollowUp creates a customer follow-up email
func (g *Generator) generateCustomerFollowUp(caseObj *servicedomain.Case, customer *Contact, scenario *CaseScenario, exchangeNum int, timestamp time.Time) *servicedomain.InboundEmail {
	// Pick follow-up template
	followUp := scenario.CustomerFollowUps[exchangeNum%len(scenario.CustomerFollowUps)]

	return &servicedomain.InboundEmail{
		ID:          id.New(),
		WorkspaceID: caseObj.WorkspaceID,
		CaseID:      caseObj.ID,
		MessageID:   fmt.Sprintf("<%s@mail.example.com>", id.New()[:16]),
		FromEmail:   customer.Email,
		FromName:    customer.Name,
		ToEmails:    []string{"support@mbr.local"},
		Subject:     fmt.Sprintf("Re: %s", caseObj.Subject),
		TextContent: followUp,
		HTMLContent: fmt.Sprintf("<p>%s</p>", followUp),
		ReceivedAt:  timestamp,
		ProcessedAt: &timestamp,
		CreatedAt:   timestamp,
	}
}

// determineCaseStatus determines status based on age
func (g *Generator) determineCaseStatus(daysAgo int) servicedomain.CaseStatus {
	// Older cases more likely to be closed
	if daysAgo > 14 {
		r := g.rand.Float32()
		if r < 0.5 {
			return servicedomain.CaseStatusClosed
		} else if r < 0.8 {
			return servicedomain.CaseStatusResolved
		}
	}
	if daysAgo > 7 {
		r := g.rand.Float32()
		if r < 0.3 {
			return servicedomain.CaseStatusClosed
		} else if r < 0.6 {
			return servicedomain.CaseStatusResolved
		} else if r < 0.8 {
			return servicedomain.CaseStatusPending
		}
	}
	// Recent cases
	r := g.rand.Float32()
	if r < 0.3 {
		return servicedomain.CaseStatusNew
	} else if r < 0.6 {
		return servicedomain.CaseStatusOpen
	}
	return servicedomain.CaseStatusPending
}

// randomPerson returns a random person name
func (g *Generator) randomPerson() Person {
	return Person{
		FirstName: firstNames[g.rand.Intn(len(firstNames))],
		LastName:  lastNames[g.rand.Intn(len(lastNames))],
	}
}

// log outputs a log message if verbose mode is enabled
func (g *Generator) log(format string, args ...interface{}) {
	if g.config.Verbose {
		fmt.Printf("[synth] "+format+"\n", args...)
	}
}

// Person represents a generated person
type Person struct {
	FirstName string
	LastName  string
}

// FullName returns the full name
func (p Person) FullName() string {
	return fmt.Sprintf("%s %s", p.FirstName, p.LastName)
}
