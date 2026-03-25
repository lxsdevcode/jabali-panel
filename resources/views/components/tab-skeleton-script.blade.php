<style>
@keyframes tab-skeleton-pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
}
</style>
<script>
document.addEventListener('livewire:init', () => {
    const isDark = () => document.documentElement.classList.contains('dark');

    function createSkeletonBar(widthPercent) {
        const bar = document.createElement('div');
        bar.style.cssText = `height:1rem;width:${widthPercent};border-radius:0.25rem;animation:tab-skeleton-pulse 2s ease-in-out infinite;background:${isDark() ? 'rgba(255,255,255,0.1)' : '#e5e7eb'};`;
        return bar;
    }

    function getOrCreateSkeleton(container) {
        let skeleton = container.querySelector('[data-tab-skeleton]');
        if (skeleton) {
            skeleton.querySelectorAll('div').forEach(bar => {
                bar.style.background = isDark() ? 'rgba(255,255,255,0.1)' : '#e5e7eb';
            });
            return skeleton;
        }
        skeleton = document.createElement('div');
        skeleton.setAttribute('data-tab-skeleton', '');
        skeleton.style.cssText = 'padding:2rem 1.5rem;display:none;flex-direction:column;gap:0.75rem;';
        skeleton.appendChild(createSkeletonBar('75%'));
        skeleton.appendChild(createSkeletonBar('100%'));
        skeleton.appendChild(createSkeletonBar('66%'));
        skeleton.appendChild(createSkeletonBar('83%'));
        return skeleton;
    }

    Livewire.hook('commit', ({ commit, succeed }) => {
        const updates = Object.keys(commit.updates || {});
        if (!updates.includes('activeTab') && !updates.includes('viewMode')) return;

        document.querySelectorAll('.fi-sc-tabs').forEach(container => {
            const tabBar = container.querySelector('.fi-tabs');
            if (!tabBar) return;

            let sibling = tabBar.nextElementSibling;
            while (sibling) {
                if (!sibling.hasAttribute('data-tab-skeleton')) {
                    sibling.style.display = 'none';
                }
                sibling = sibling.nextElementSibling;
            }

            const skeleton = getOrCreateSkeleton(container);
            tabBar.after(skeleton);
            skeleton.style.display = 'flex';
        });

        succeed(() => {
            document.querySelectorAll('.fi-sc-tabs').forEach(container => {
                const tabBar = container.querySelector('.fi-tabs');
                if (!tabBar) return;

                let sibling = tabBar.nextElementSibling;
                while (sibling) {
                    if (!sibling.hasAttribute('data-tab-skeleton')) {
                        sibling.style.display = '';
                    }
                    sibling = sibling.nextElementSibling;
                }

                const skeleton = container.querySelector('[data-tab-skeleton]');
                if (skeleton) skeleton.style.display = 'none';
            });
        });
    });
});
</script>
