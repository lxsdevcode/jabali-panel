@props(['target' => 'activeTab'])

<div x-data="{ loading: false }" x-init="
    const wireId = $wire.__instance?.id;
    if (!wireId) return;

    Livewire.hook('commit', ({ component, commit, succeed }) => {
        if (component.id !== wireId) return;
        if (!commit.updates || !commit.updates.hasOwnProperty('{{ $target }}')) return;
        loading = true;
        succeed(() => { loading = false; });
    });
">
    <div x-show="loading" x-cloak class="py-8 space-y-3">
        <div class="h-4 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-full animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-2/3 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-5/6 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
    </div>
    <div x-show="!loading" {{ $attributes }}>
        {{ $slot }}
    </div>
</div>
