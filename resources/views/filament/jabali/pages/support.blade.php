<x-filament-panels::page>
    <div class="grid gap-6 md:grid-cols-2">
        <x-filament::section
            icon="heroicon-o-book-open"
            icon-color="primary"
        >
            <x-slot name="heading">{{ __('Documentation') }}</x-slot>
            <x-slot name="description">{{ __('Find answers in our docs or talk with our trained support bot. Explore setup guides, troubleshooting steps, and best practices.') }}</x-slot>

            <div class="flex justify-center">
                <x-filament::button
                    tag="a"
                    href="https://jabali-panel.com/docs/"
                    target="_blank"
                    rel="noopener"
                    icon="heroicon-o-arrow-top-right-on-square"
                >
                    {{ __('Open Documentation') }}
                </x-filament::button>
            </div>
        </x-filament::section>

        <x-filament::section
            icon="heroicon-o-bug-ant"
            icon-color="warning"
        >
            <x-slot name="heading">{{ __('GitHub Issues') }}</x-slot>
            <x-slot name="description">{{ __('Report bugs or request features. Include steps, logs, and screenshots so we can reproduce quickly.') }}</x-slot>

            <div class="flex justify-center">
                <x-filament::button
                    tag="a"
                    href="https://github.com/shukiv/jabali-panel/issues"
                    target="_blank"
                    rel="noopener"
                    icon="heroicon-o-arrow-top-right-on-square"
                    color="gray"
                >
                    {{ __('Open GitHub Issues') }}
                </x-filament::button>
            </div>
        </x-filament::section>
    </div>
</x-filament-panels::page>
