// EML Viewer - Client-side JavaScript

// Toast notification system
function showToast(message, type = 'info', duration = 3000) {
    const container = document.getElementById('toast-container');

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;

    container.appendChild(toast);

    setTimeout(() => {
        toast.style.animation = 'slideOut 0.3s ease-in';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

// Keyboard shortcuts
document.addEventListener('keydown', (e) => {
    // Ctrl/Cmd + K: Focus search
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        const searchInput = document.getElementById('search-input');
        if (searchInput) {
            searchInput.focus();
            searchInput.select();
        }
    }

    // Escape: Clear search
    if (e.key === 'Escape') {
        const searchInput = document.getElementById('search-input');
        if (searchInput && document.activeElement === searchInput) {
            searchInput.value = '';
            searchInput.dispatchEvent(new Event('search'));
        }
    }
});

// Copy to clipboard helper
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        showToast('Copied to clipboard!', 'success', 2000);
    }).catch(() => {
        showToast('Failed to copy', 'error', 2000);
    });
}

// Make URLs clickable in plain text emails
function linkifyText() {
    const textContent = document.querySelectorAll('pre');
    textContent.forEach(pre => {
        const urlPattern = /(https?:\/\/[^\s]+)/g;
        pre.innerHTML = pre.textContent.replace(urlPattern, '<a href="$1" target="_blank" class="text-blue-600 hover:underline">$1</a>');
    });
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    linkifyText();

    // Log keyboard shortcuts hint
    console.log('Keyboard shortcuts:');
    console.log('  Ctrl/Cmd + K: Focus search');
    console.log('  Escape: Clear search');
});

// HTMX event listeners
document.body.addEventListener('htmx:beforeRequest', (event) => {
    console.log('Request starting:', event.detail.path);
});

document.body.addEventListener('htmx:afterRequest', (event) => {
    console.log('Request completed:', event.detail.path);

    // Re-linkify text after dynamic content loads
    linkifyText();
});

document.body.addEventListener('htmx:responseError', (event) => {
    console.error('Request failed:', event.detail);
    showToast('Failed to load data', 'error');
});
