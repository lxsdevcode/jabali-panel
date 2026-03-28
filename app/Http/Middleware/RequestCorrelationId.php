<?php

declare(strict_types=1);

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Str;
use Symfony\Component\HttpFoundation\Response;

class RequestCorrelationId
{
    public function handle(Request $request, Closure $next): Response
    {
        $correlationId = $request->header('X-Request-ID') ?? Str::uuid()->toString();

        Log::shareContext(['correlation_id' => $correlationId]);

        $response = $next($request);

        $response->headers->set('X-Request-ID', $correlationId);

        return $response;
    }
}
