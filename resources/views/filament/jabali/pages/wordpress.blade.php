<x-filament-panels::page>
    {{-- Info Banner --}}
    <x-filament::section
        icon="heroicon-o-information-circle"
        icon-color="info"
    >
        <x-slot name="heading">{{ __('Tip') }}</x-slot>
        <x-slot name="description">{{ __('Jabali Panel includes the Jabali Cache plugin for optimal caching performance. Enable it via the Cache toggle in your site\'s settings. After installing WordPress, complete the setup wizard, install security plugins, and keep everything updated.') }}</x-slot>
    </x-filament::section>

    {{ $this->table }}

    <x-filament-actions::modals />

    @script
    <script>
        $wire.on('open-url', ({ url }) => {
            window.open(url, '_blank');
        });
    </script>
    @endscript
</x-filament-panels::page>
