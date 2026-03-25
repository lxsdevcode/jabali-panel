<div
    id="loading-skeleton"
    class="fixed inset-0 z-40 flex items-center justify-center bg-white/60 backdrop-blur-[1px] dark:bg-gray-900/60 transition-opacity duration-200"
    style="display: none; opacity: 0;"
>
    <div class="flex w-full max-w-2xl flex-col gap-3 px-8">
        <div class="h-4 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-full animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
        <div class="h-4 w-2/3 animate-pulse rounded bg-gray-200 dark:bg-white/10"></div>
    </div>
</div>

<script>
document.addEventListener('livewire:init', () => {
    let timer = null
    const el = document.getElementById('loading-skeleton')

    Livewire.hook('request', ({ respond, fail }) => {
        timer = setTimeout(() => {
            el.style.display = 'flex'
            requestAnimationFrame(() => { el.style.opacity = '1' })
        }, 200)

        const hide = () => {
            clearTimeout(timer)
            el.style.opacity = '0'
            setTimeout(() => { el.style.display = 'none' }, 200)
        }

        respond(hide)
        fail(hide)
    })
})
</script>
