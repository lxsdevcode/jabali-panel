<?php

declare(strict_types=1);

namespace App\Http\Middleware;

use Closure;
use Filament\Facades\Filament;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

class EnforceTwoFactor
{
    /**
     * Require admin users to configure 2FA before accessing the panel.
     *
     * @param  Closure(Request): (Response)  $next
     */
    public function handle(Request $request, Closure $next): Response
    {
        $user = Filament::auth()->user();

        if (! $user || ! $user->is_admin) {
            return $next($request);
        }

        $setupPath = Filament::getPanel('admin')->getPath().'/two-factor-setup';

        // Allow access to the 2FA setup page itself and auth routes
        if ($request->is($setupPath) || $request->routeIs('filament.admin.auth.*')) {
            return $next($request);
        }

        // If admin has no 2FA configured, redirect to setup
        if (! $user->two_factor_secret || ! $user->two_factor_confirmed_at) {
            return redirect('/'.$setupPath);
        }

        return $next($request);
    }
}
