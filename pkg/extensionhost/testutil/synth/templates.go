package synth

import servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"

// Workspace name templates
type WorkspaceName struct {
	Name        string
	Slug        string
	Description string
}

var workspaceNames = []WorkspaceName{
	{
		Name:        "Acme Corp Support",
		Slug:        "acme-corp",
		Description: "Customer support for Acme Corporation products and services",
	},
	{
		Name:        "TechStart Solutions",
		Slug:        "techstart",
		Description: "Technical support for TechStart SaaS platform",
	},
	{
		Name:        "CloudBase Inc",
		Slug:        "cloudbase",
		Description: "Support desk for CloudBase infrastructure services",
	},
	{
		Name:        "DataFlow Analytics",
		Slug:        "dataflow",
		Description: "Customer success for DataFlow analytics platform",
	},
	{
		Name:        "SecureAuth Systems",
		Slug:        "secureauth",
		Description: "Enterprise security product support",
	},
}

// Team templates
type TeamTemplate struct {
	Name        string
	Slug        string
	Description string
}

var teamTemplates = []TeamTemplate{
	{Name: "General Support", Slug: "general", Description: "General customer inquiries"},
	{Name: "Technical Support", Slug: "technical", Description: "Technical issues and bugs"},
	{Name: "Billing", Slug: "billing", Description: "Billing and payment issues"},
	{Name: "Enterprise", Slug: "enterprise", Description: "Enterprise customer support"},
}

// Company templates for generating customers
type Company struct {
	Name   string
	Domain string
}

var companies = []Company{
	{Name: "Globex Corporation", Domain: "globex.com"},
	{Name: "Initech", Domain: "initech.com"},
	{Name: "Umbrella Corp", Domain: "umbrella.co"},
	{Name: "Stark Industries", Domain: "stark.io"},
	{Name: "Wayne Enterprises", Domain: "wayne.com"},
	{Name: "Cyberdyne Systems", Domain: "cyberdyne.net"},
	{Name: "Tyrell Corporation", Domain: "tyrell.com"},
	{Name: "Weyland-Yutani", Domain: "weyland.co"},
	{Name: "Massive Dynamic", Domain: "massive-dynamic.com"},
	{Name: "Hooli", Domain: "hooli.xyz"},
	{Name: "Pied Piper", Domain: "piedpiper.com"},
	{Name: "Aviato", Domain: "aviato.com"},
	{Name: "Dunder Mifflin", Domain: "dundermifflin.com"},
	{Name: "Sterling Cooper", Domain: "sterlingcooper.com"},
	{Name: "Vandelay Industries", Domain: "vandelay.com"},
}

// Name data
var firstNames = []string{
	"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda",
	"William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Charles", "Karen", "Christopher", "Nancy", "Daniel", "Lisa",
	"Matthew", "Betty", "Anthony", "Margaret", "Mark", "Sandra", "Donald", "Ashley",
	"Steven", "Kimberly", "Paul", "Emily", "Andrew", "Donna", "Joshua", "Michelle",
	"Kenneth", "Dorothy", "Kevin", "Carol", "Brian", "Amanda", "George", "Melissa",
	"Alex", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Jamie", "Quinn",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
	"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson",
	"White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker",
	"Young", "Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill",
	"Flores", "Green", "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell",
	"Mitchell", "Carter", "Roberts", "Chen", "Kim", "Patel", "Singh", "Kumar",
}

// CaseScenario represents a realistic support case scenario
type CaseScenario struct {
	Category          string
	Subject           string
	Priority          servicedomain.CasePriority
	Tags              []string
	InitialMessage    string
	AgentResponses    []string
	CustomerFollowUps []string
}

var caseScenarios = []*CaseScenario{
	// Login/Authentication Issues
	{
		Category: "authentication",
		Subject:  "Unable to log in to my account",
		Priority: servicedomain.CasePriorityHigh,
		Tags:     []string{"login", "authentication", "access"},
		InitialMessage: `Hi,

I've been trying to log in to my account for the past hour but keep getting an "Invalid credentials" error. I'm 100% sure my password is correct because I just reset it yesterday.

I've tried:
- Clearing my browser cache
- Using incognito mode
- Trying a different browser

Nothing works. I have an important deadline today and really need access to my account.

My email is the one I'm sending from. Please help urgently!

Thanks`,
		AgentResponses: []string{
			`Thank you for reaching out. I understand how frustrating login issues can be, especially when you have urgent work to complete.

I've checked your account and I can see there were multiple failed login attempts. As a security measure, your account was temporarily locked.

I've unlocked your account now. Please try logging in again using the password reset link I'm sending to your email. Make sure to check your spam folder if you don't see it within a few minutes.

Let me know if you're able to get in!`,
			`I'm glad we got you back into your account! To prevent this from happening again, I'd recommend:

1. Enable two-factor authentication for added security
2. Use a password manager to avoid typos
3. Bookmark the correct login URL

Is there anything else I can help you with today?`,
		},
		CustomerFollowUps: []string{
			`That worked! I'm in now. Thank you so much for the quick response.

One question - how do I enable that two-factor authentication you mentioned?`,
			`Perfect, I've set up 2FA now. Thanks again for all your help!`,
		},
	},

	// Billing Issue
	{
		Category: "billing",
		Subject:  "Charged twice for my subscription",
		Priority: servicedomain.CasePriorityHigh,
		Tags:     []string{"billing", "payment", "refund"},
		InitialMessage: `Hello,

I just noticed that I was charged twice for my monthly subscription this month. I can see two charges of $49.99 on my credit card statement dated the 1st and 3rd of this month.

Order IDs from my account:
- ORD-2024-001234
- ORD-2024-001256

This is concerning and I'd like one of these charges refunded immediately.

Please investigate and let me know what happened.

Regards`,
		AgentResponses: []string{
			`I sincerely apologize for this billing error. I've investigated your account and can confirm that you were indeed charged twice due to a payment processing glitch on our end.

I've initiated a refund of $49.99 for the duplicate charge (ORD-2024-001256). You should see this reflected in your account within 3-5 business days, depending on your bank.

For your inconvenience, I've also applied a 20% discount to your next month's subscription.

Again, I apologize for this error. Is there anything else I can help you with?`,
			`You're very welcome! The refund should appear on your statement soon. If you don't see it within 5 business days, please don't hesitate to reach out and I'll follow up with our payment processor.

Thank you for your patience and understanding!`,
		},
		CustomerFollowUps: []string{
			`Thank you for the quick resolution and the discount - that's very generous of you.

I'll keep an eye on my statement for the refund.`,
		},
	},

	// Feature Request
	{
		Category: "feature-request",
		Subject:  "Request: Dark mode for the dashboard",
		Priority: servicedomain.CasePriorityLow,
		Tags:     []string{"feature-request", "ui", "accessibility"},
		InitialMessage: `Hi there,

I love using your platform, but I often work late at night and the bright interface is really straining my eyes.

Would it be possible to add a dark mode option? Many of your competitors already have this feature and it would really improve my experience.

Thanks for considering!`,
		AgentResponses: []string{
			`Thank you for this suggestion! We really appreciate users taking the time to share their ideas with us.

I'm happy to let you know that dark mode is actually on our product roadmap! Our development team is currently working on it and we expect to release it in the next quarter.

I've added your vote to this feature request to help prioritize it. Would you like me to notify you when it becomes available?`,
			`Great! I've added you to the notification list. You'll receive an email as soon as dark mode is released.

In the meantime, you might want to try browser extensions like "Dark Reader" which can apply a dark theme to websites. It's not perfect but it can help reduce eye strain.

Thanks again for your feedback!`,
		},
		CustomerFollowUps: []string{
			`Yes, please add me to the notification list! That's great news that it's already being worked on.

I'll try that browser extension in the meantime. Thanks for the tip!`,
		},
	},

	// Technical Bug
	{
		Category: "bug",
		Subject:  "Export to PDF not working - blank pages",
		Priority: servicedomain.CasePriorityMedium,
		Tags:     []string{"bug", "export", "pdf"},
		InitialMessage: `Hi Support,

I'm trying to export my reports to PDF but every time I do, I get a file with blank pages. The export process completes without any error messages, but when I open the PDF, all pages are empty.

Browser: Chrome 120.0
OS: Windows 11
Report: Monthly Sales Summary for December 2024

I've attached a screenshot of what I see.

This is blocking my work as I need to send these reports to stakeholders.`,
		AgentResponses: []string{
			`Thank you for the detailed bug report. I've been able to reproduce this issue and have escalated it to our engineering team.

This appears to be related to a recent Chrome update that changed how PDFs are rendered. We're working on a fix.

In the meantime, here are two workarounds:
1. Try exporting in Firefox or Edge - both work correctly
2. Use the "Print to PDF" option instead of the direct export

I'll update you as soon as we have a permanent fix deployed. I apologize for the inconvenience.`,
			`Good news! Our engineering team has deployed a fix for the PDF export issue. Could you please try exporting again and let me know if it works correctly now?

If you're still seeing blank pages, please try clearing your browser cache first.`,
			`Excellent! I'm glad to hear the fix worked. Thank you for your patience while we resolved this.

We've also added additional testing for PDF exports to prevent similar issues in the future.

Is there anything else I can help you with?`,
		},
		CustomerFollowUps: []string{
			`Thanks for the quick response. I tried Firefox and the export works there.

Looking forward to the permanent fix though - Chrome is my main browser.`,
			`Just tested it in Chrome - working perfectly now! Thanks for the fast turnaround on this fix.`,
		},
	},

	// Onboarding Help
	{
		Category: "onboarding",
		Subject:  "New user - need help getting started",
		Priority: servicedomain.CasePriorityMedium,
		Tags:     []string{"onboarding", "new-user", "setup"},
		InitialMessage: `Hello,

I just signed up for your service yesterday and I'm feeling a bit overwhelmed. There are so many features and I'm not sure where to start.

Could you point me to some getting started guides or maybe schedule a quick onboarding call?

I'm particularly interested in:
- Setting up my first project
- Inviting team members
- Connecting integrations

Thanks!`,
		AgentResponses: []string{
			`Welcome aboard! It's completely normal to feel overwhelmed at first - we have a lot of powerful features.

Here are some resources to help you get started:

1. **Quick Start Guide**: https://docs.example.com/quickstart
2. **Video Tutorials**: https://example.com/tutorials
3. **Interactive Tour**: Click the "?" icon in the dashboard

I'd also be happy to schedule a 30-minute onboarding call with you. We offer these for free to all new users. Would tomorrow at 2 PM EST work for you?`,
			`Perfect! I've scheduled your onboarding call for tomorrow at 2 PM EST. You'll receive a calendar invite shortly with the video call link.

Before the call, I'd recommend:
1. Exploring the dashboard briefly
2. Writing down any specific questions you have
3. Having your team members' email addresses ready if you want to add them

Looking forward to speaking with you!`,
		},
		CustomerFollowUps: []string{
			`Thank you! Tomorrow at 2 PM EST works perfectly for me.

I'll check out those resources before the call.`,
			`The onboarding call was incredibly helpful! I feel much more confident using the platform now. Thank you!`,
		},
	},

	// Integration Issue
	{
		Category: "integration",
		Subject:  "Slack integration stopped sending notifications",
		Priority: servicedomain.CasePriorityMedium,
		Tags:     []string{"integration", "slack", "notifications"},
		InitialMessage: `Hi,

Our Slack integration was working fine until last week, but now we're not receiving any notifications in our #alerts channel.

I've checked:
- The integration shows as "Connected" in settings
- The channel still exists
- No error messages in the integration logs

We rely on these notifications for critical alerts. Please help!`,
		AgentResponses: []string{
			`I understand how important these notifications are for your workflow. Let me help you troubleshoot this.

Based on your description, this sounds like it might be a Slack token expiration issue. Slack recently changed their token policies and some integrations need to be reauthorized.

Could you try the following:
1. Go to Settings > Integrations > Slack
2. Click "Disconnect"
3. Wait 30 seconds
4. Click "Reconnect" and reauthorize

This should refresh your Slack token. Let me know if notifications start working after this.`,
			`Glad that resolved the issue! For reference, Slack tokens now expire after 12 hours of inactivity by default. Since you reconnected, your integration should now use a new refresh mechanism we implemented to handle this automatically.

If you notice notifications stopping again in the future, please let us know immediately. Is there anything else I can help with?`,
		},
		CustomerFollowUps: []string{
			`That worked! I disconnected and reconnected, and notifications are flowing again.

Is there any way to prevent this from happening again? We can't afford to miss critical alerts.`,
		},
	},

	// Performance Issue
	{
		Category: "performance",
		Subject:  "Dashboard loading extremely slowly",
		Priority: servicedomain.CasePriorityHigh,
		Tags:     []string{"performance", "slow", "dashboard"},
		InitialMessage: `Hello,

For the past few days, our dashboard has been incredibly slow to load. It used to load in 2-3 seconds, now it takes 30+ seconds.

This is happening for all users on our team, not just me. We're on the Enterprise plan and this performance is unacceptable for what we're paying.

Please investigate urgently.`,
		AgentResponses: []string{
			`I apologize for the performance issues you're experiencing. This is definitely not the experience we want for our Enterprise customers.

I've immediately escalated this to our infrastructure team. Looking at your account, I can see you have a large dataset (500,000+ records) which may be contributing to the slowdown.

We're investigating whether this is:
1. A recent code regression
2. Database optimization needed for your specific data patterns
3. Infrastructure scaling issue

I'll update you within the next 2 hours with our findings.`,
			`Update: Our team has identified the root cause. A recent update introduced inefficient queries for accounts with large datasets like yours.

We've deployed a hotfix and also added additional database indexes optimized for your usage patterns.

Could you try loading your dashboard now and let me know if you see improvement? Based on our tests, load times should be back to under 3 seconds.`,
		},
		CustomerFollowUps: []string{
			`Thank you for the quick escalation. Looking forward to hearing what you find.`,
			`Just tested - loading in about 2 seconds now! That's a huge improvement. Thank you for fixing this so quickly.`,
		},
	},

	// Account Cancellation
	{
		Category: "account",
		Subject:  "Request to cancel my subscription",
		Priority: servicedomain.CasePriorityMedium,
		Tags:     []string{"cancellation", "account", "churn"},
		InitialMessage: `Hi,

I'd like to cancel my subscription effective at the end of this billing cycle.

We've decided to go with a different solution that better fits our current needs.

Please confirm the cancellation and let me know if there's anything I need to do on my end.

Thanks`,
		AgentResponses: []string{
			`I'm sorry to hear you're leaving us. Before I process the cancellation, may I ask what features or capabilities you're looking for that we weren't able to provide?

Your feedback would be incredibly valuable to help us improve.

Also, I wanted to let you know about a few things:
1. We recently launched several new features you might not be aware of
2. We could offer a 3-month discount while you evaluate
3. I'd be happy to schedule a call to discuss your specific needs

Of course, if you've made your decision, I completely respect that and will process the cancellation immediately.`,
			`I understand. I've processed your cancellation request. Your subscription will remain active until the end of your current billing period on January 15th.

A few things to note:
- You can export your data anytime before then
- Your account will be retained in read-only mode for 30 days after cancellation
- You're welcome to reactivate anytime within that period

Thank you for being a customer. We hope to see you again in the future!`,
		},
		CustomerFollowUps: []string{
			`Thanks for the offer, but we've already committed to the other solution. The main issue was the lack of native mobile apps - our team works in the field a lot.

If you add mobile apps in the future, we'd definitely consider coming back.`,
		},
	},
}
