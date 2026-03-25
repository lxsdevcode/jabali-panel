<div id="loading-bar" class="fixed top-0 inset-x-0 z-50 h-0.5" style="display: none;">
    <div id="loading-bar-inner" class="h-full bg-primary-500 rounded-r-full" style="width: 0%; transition: width 0.3s ease;"></div>
</div>

<script>
document.addEventListener('livewire:init', () => {
    let timer = null
    let interval = null
    const bar = document.getElementById('loading-bar')
    const inner = document.getElementById('loading-bar-inner')

    Livewire.hook('request', ({ respond, fail }) => {
        let width = 15

        timer = setTimeout(() => {
            inner.style.width = '0%'
            bar.style.display = 'block'

            requestAnimationFrame(() => {
                inner.style.width = width + '%'
            })

            interval = setInterval(() => {
                width += Math.random() * 10
                if (width > 90) width = 90
                inner.style.width = width + '%'
            }, 300)
        }, 100)

        const hide = () => {
            clearTimeout(timer)
            clearInterval(interval)
            inner.style.width = '100%'
            setTimeout(() => {
                bar.style.display = 'none'
                inner.style.width = '0%'
            }, 300)
        }

        respond(hide)
        fail(hide)
    })
})
</script>
