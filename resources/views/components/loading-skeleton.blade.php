<div
    x-data="{ loading: false, timer: null }"
    x-on:livewire:request.window="
        clearTimeout(timer);
        timer = setTimeout(() => { loading = true }, 200);
        $event.detail.respond(() => { clearTimeout(timer); loading = false });
    "
    x-show="loading"
    x-transition:enter="transition ease-out duration-200"
    x-transition:enter-start="opacity-0"
    x-transition:enter-end="opacity-100"
    x-transition:leave="transition ease-in duration-150"
    x-transition:leave-start="opacity-100"
    x-transition:leave-end="opacity-0"
    class="fixed inset-0 z-40 flex items-center justify-center bg-white/60 backdrop-blur-[1px] dark:bg-gray-900/60"
    style="display: none;"
>
    <div class="flex w-full max-w-2xl flex-col gap-3 px-8">
        <div class="h-4 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-full animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-2/3 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
    </div>
</div>
