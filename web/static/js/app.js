// Custom JavaScript for Move Big Rocks

// Toast notification helper
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `alert alert-${type} shadow-lg`;
    toast.innerHTML = `
        <div>
            <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current flex-shrink-0 h-6 w-6" fill="none" viewBox="0 0 24 24">
                ${type === 'success' ? '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />' : ''}
                ${type === 'error' ? '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />' : ''}
                ${type === 'info' ? '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />' : ''}
            </svg>
            <span>${message}</span>
        </div>
    `;

    const container = document.getElementById('toast-container');
    container.appendChild(toast);

    // Auto-remove after 5 seconds
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transition = 'opacity 300ms';
        setTimeout(() => toast.remove(), 300);
    }, 5000);
}

// Listen for HTMX events
document.body.addEventListener('htmx:afterSwap', function(event) {
    // Auto-focus first input in swapped content
    const firstInput = event.detail.target.querySelector('input, textarea, select');
    if (firstInput) {
        firstInput.focus();
    }
});

document.body.addEventListener('htmx:responseError', function(event) {
    showToast('An error occurred. Please try again.', 'error');
});

document.body.addEventListener('htmx:sendError', function(event) {
    showToast('Network error. Please check your connection.', 'error');
});

// Listen for custom events
document.body.addEventListener('messageSent', function(event) {
    showToast('Message sent successfully', 'success');
});

document.body.addEventListener('caseUpdated', function(event) {
    showToast('Case updated successfully', 'success');
});

document.body.addEventListener('issueResolved', function(event) {
    showToast('Issue marked as resolved', 'success');
});

// Keyboard shortcuts
document.addEventListener('keydown', function(event) {
    // Cmd/Ctrl + K for quick search
    if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
        event.preventDefault();
        const searchInput = document.querySelector('input[type="search"]');
        if (searchInput) {
            searchInput.focus();
        }
    }

    // Escape to close modals
    if (event.key === 'Escape') {
        const modal = document.querySelector('.modal[open]');
        if (modal) {
            modal.close();
        }
    }
});

// Select all checkbox functionality
document.addEventListener('change', function(event) {
    if (event.target.id === 'select-all') {
        const checkboxes = document.querySelectorAll('tbody input[type="checkbox"]');
        checkboxes.forEach(cb => cb.checked = event.target.checked);
    }
});

// Auto-save draft functionality (for forms)
let draftTimeout;
function saveDraft(formId, data) {
    localStorage.setItem(`draft_${formId}`, JSON.stringify(data));
}

function loadDraft(formId) {
    const draft = localStorage.getItem(`draft_${formId}`);
    return draft ? JSON.parse(draft) : null;
}

function clearDraft(formId) {
    localStorage.removeItem(`draft_${formId}`);
}

// Restore drafts on page load
document.addEventListener('DOMContentLoaded', function() {
    // Restore form drafts if they exist
    const forms = document.querySelectorAll('form[data-save-draft]');
    forms.forEach(form => {
        const draft = loadDraft(form.id);
        if (draft) {
            Object.keys(draft).forEach(key => {
                const input = form.querySelector(`[name="${key}"]`);
                if (input) {
                    input.value = draft[key];
                }
            });
        }

        // Auto-save on input
        form.addEventListener('input', function(e) {
            clearTimeout(draftTimeout);
            draftTimeout = setTimeout(() => {
                const data = {};
                new FormData(form).forEach((value, key) => {
                    data[key] = value;
                });
                saveDraft(form.id, data);
            }, 1000);
        });

        // Clear draft on successful submit
        form.addEventListener('htmx:afterRequest', function(event) {
            if (event.detail.successful) {
                clearDraft(form.id);
            }
        });
    });
});

// Markdown preview toggle
function toggleMarkdownPreview(textareaId, previewId) {
    const textarea = document.getElementById(textareaId);
    const preview = document.getElementById(previewId);
    const isPreview = preview.style.display !== 'none';

    if (isPreview) {
        preview.style.display = 'none';
        textarea.style.display = 'block';
    } else {
        // Simple markdown to HTML (basic implementation)
        const html = textarea.value
            .replace(/^### (.*$)/gim, '<h3>$1</h3>')
            .replace(/^## (.*$)/gim, '<h2>$1</h2>')
            .replace(/^# (.*$)/gim, '<h1>$1</h1>')
            .replace(/\*\*(.*?)\*\*/gim, '<strong>$1</strong>')
            .replace(/\*(.*?)\*/gim, '<em>$1</em>')
            .replace(/\n/gim, '<br>');

        preview.innerHTML = html;
        preview.style.display = 'block';
        textarea.style.display = 'none';
    }
}

// Theme switcher (if you want to add dark mode toggle)
function toggleTheme() {
    const html = document.documentElement;
    const currentTheme = html.getAttribute('data-theme');
    const newTheme = currentTheme === 'light' ? 'dark' : 'light';
    html.setAttribute('data-theme', newTheme);
    localStorage.setItem('theme', newTheme);
}

// Load saved theme
const savedTheme = localStorage.getItem('theme') || 'light';
document.documentElement.setAttribute('data-theme', savedTheme);

// Confirmation dialogs
document.addEventListener('htmx:confirm', function(event) {
    if (event.detail.question && !confirm(event.detail.question)) {
        event.preventDefault();
    }
});

console.log('Move Big Rocks app.js loaded ✨');
