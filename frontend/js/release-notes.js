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

        const fragment = document.createDocumentFragment();
        releases.forEach(release => {
            const isCurrent = release.version === meta.version;
            const section = document.createElement('section');
            section.className = `release-card${isCurrent ? ' current' : ''}`;

            const title = document.createElement('h2');
            title.textContent = release.version || 'unknown version';

            const date = document.createElement('div');
            date.className = 'release-date';
            date.textContent = release.released_at || 'unknown date';

            const summary = document.createElement('p');
            summary.className = 'release-summary';
            summary.textContent = release.summary || '';

            const changes = document.createElement('ul');
            (release.changes || []).forEach(change => {
                const item = document.createElement('li');
                item.textContent = change;
                changes.appendChild(item);
            });

            section.append(title, date, summary, changes);
            fragment.appendChild(section);
        });
        listEl.replaceChildren(fragment);

        stateEl.hidden = true;
        listEl.hidden = false;
    } catch (error) {
        console.error('Error loading release notes:', error);
        stateEl.textContent = 'Release notes are temporarily unavailable.';
    }
}
