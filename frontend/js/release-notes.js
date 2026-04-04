document.addEventListener('DOMContentLoaded', () => {
    loadReleaseNotes();
});

async function loadReleaseNotes() {
    const stateEl = document.getElementById('release-notes-state');
    const listEl = document.getElementById('release-notes-list');

    try {
        const metaResp = await fetch(`${AppConfig.apiBaseUrl}/api/meta`);
        if (!metaResp.ok) {
            throw new Error('Failed to fetch app metadata');
        }
        const meta = await metaResp.json();

        const changelogResp = await fetch(`${AppConfig.apiBaseUrl}/api/changelog`);
        if (!changelogResp.ok) {
            throw new Error('Failed to fetch release notes');
        }
        const changelog = await changelogResp.json();

        const releases = changelog.releases || [];
        if (releases.length === 0) {
            stateEl.textContent = 'No release notes available yet.';
            return;
        }

        listEl.innerHTML = releases.map(release => {
            const isCurrent = release.version === meta.version;
            const changes = (release.changes || [])
                .map(change => `<li>${escapeHTML(change)}</li>`)
                .join('');

            return `
                <section class="release-card${isCurrent ? ' current' : ''}">
                    <h2>${escapeHTML(release.version)}</h2>
                    <div class="release-date">${escapeHTML(release.released_at || 'unknown date')}</div>
                    <p class="release-summary">${escapeHTML(release.summary || '')}</p>
                    <ul>${changes}</ul>
                </section>
            `;
        }).join('');

        stateEl.hidden = true;
        listEl.hidden = false;
    } catch (error) {
        console.error('Error loading release notes:', error);
        stateEl.textContent = 'Release notes are temporarily unavailable.';
    }
}

function escapeHTML(value) {
    return String(value)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}
