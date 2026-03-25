@props(['target' => 'activeTab'])

<div
    x-data="{ loading: false }"
    x-init="
        Livewire.hook('commit', ({ component, commit, respond }) => {
            if (component.id !== $wire.__instance.id) return;
            if (!commit.updates || !commit.updates.hasOwnProperty('{{ $target }}')) return;
            loading = true;
            respond(() => { loading = false; });
        })
    "
>
    <template x-if="loading">
        <div class="py-8 space-y-3">
            <div class="h-4 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
            <div class="h-4 w-full animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
            <div class="h-4 w-2/3 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
            <div class="h-4 w-5/6 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        </div>
    </template>
    <div x-show="!loading" {{ $attributes }}>
        {{ $slot }}
    </div>
</div>
