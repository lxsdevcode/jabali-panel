@if(session()->has('impersonated_by'))
    @php
        $currentUser = auth()->user();
    @endphp
    @if($currentUser)
        <div class="fi-topbar-ctn flex items-center justify-center gap-x-3 bg-warning-500 px-4 py-2 dark:bg-warning-600">
            <x-filament::icon icon="heroicon-o-user" class="h-4 w-4 text-white" />
            <span class="text-sm font-medium text-white">
                {{ __('You are logged in as:') }} <strong>{{ $currentUser->name }}</strong> ({{ $currentUser->username }})
            </span>
            <form method="POST" action="{{ url('/impersonate/stop') }}">
                @csrf
                <x-filament::button size="xs" color="gray" type="submit">
                    {{ __('Return to Admin') }}
                </x-filament::button>
            </form>
        </div>
    @endif
@endif
