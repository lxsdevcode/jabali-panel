@props(['target' => 'activeTab'])

{{ $slot }}

@once
<script>
document.addEventListener('livewire:init', () => {
    function createSkeletonBar(widthPercent) {
        const bar = document.createElement('div');
        bar.style.height = '1rem';
        bar.style.width = widthPercent;
        bar.style.borderRadius = '0.25rem';
        bar.className = 'animate-pulse bg-gray-200 dark:bg-white/10';
        return bar;
    }

    function getOrCreateSkeleton(container) {
        let skeleton = container.querySelector('[data-tab-skeleton]');
        if (!skeleton) {
            skeleton = document.createElement('div');
            skeleton.setAttribute('data-tab-skeleton', '');
            skeleton.style.cssText = 'padding:2rem 0;display:none;flex-direction:column;gap:0.75rem;';
            skeleton.appendChild(createSkeletonBar('75%'));
            skeleton.appendChild(createSkeletonBar('100%'));
            skeleton.appendChild(createSkeletonBar('66%'));
            skeleton.appendChild(createSkeletonBar('83%'));
            container.appendChild(skeleton);
        }
        return skeleton;
    }

    Livewire.hook('commit', ({ commit, succeed }) => {
        const updates = Object.keys(commit.updates || {});
        if (!updates.includes('activeTab') && !updates.includes('viewMode')) return;

        document.querySelectorAll('.fi-sc-tabs').forEach(container => {
            const tabBar = container.querySelector('.fi-tabs');
            if (!tabBar) return;

            // Fade out current content
            let sibling = tabBar.nextElementSibling;
            while (sibling) {
                if (!sibling.hasAttribute('data-tab-skeleton')) {
                    sibling.style.opacity = '0.15';
                    sibling.style.pointerEvents = 'none';
                }
                sibling = sibling.nextElementSibling;
            }

            // Show skeleton
            const skeleton = getOrCreateSkeleton(container);
            skeleton.style.display = 'flex';
        });

        succeed(() => {
            document.querySelectorAll('.fi-sc-tabs').forEach(container => {
                const tabBar = container.querySelector('.fi-tabs');
                if (!tabBar) return;

                // Restore content
                let sibling = tabBar.nextElementSibling;
                while (sibling) {
                    if (!sibling.hasAttribute('data-tab-skeleton')) {
                        sibling.style.opacity = '';
                        sibling.style.pointerEvents = '';
                    }
                    sibling = sibling.nextElementSibling;
                }

                // Hide skeleton
                const skeleton = container.querySelector('[data-tab-skeleton]');
                if (skeleton) skeleton.style.display = 'none';
            });
        });
    });
});
</script>
@endonce
