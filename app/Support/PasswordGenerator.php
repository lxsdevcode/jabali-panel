<?php

declare(strict_types=1);

namespace App\Support;

class PasswordGenerator
{
    public static function generate(int $length = 16): string
    {
        $lowercase = 'abcdefghijklmnopqrstuvwxyz';
        $uppercase = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
        $numbers = '0123456789';
        $special = '!@#$%^&*';

        // Ensure at least one of each required type
        $password = $lowercase[random_int(0, strlen($lowercase) - 1)]
            .$uppercase[random_int(0, strlen($uppercase) - 1)]
            .$numbers[random_int(0, strlen($numbers) - 1)]
            .$special[random_int(0, strlen($special) - 1)];

        // Fill the rest with random characters from all types
        $allChars = $lowercase.$uppercase.$numbers.$special;
        for ($i = strlen($password); $i < $length; $i++) {
            $password .= $allChars[random_int(0, strlen($allChars) - 1)];
        }

        // Shuffle using CSPRNG (Fisher-Yates with random_int)
        $chars = str_split($password);
        for ($i = count($chars) - 1; $i > 0; $i--) {
            $j = random_int(0, $i);
            [$chars[$i], $chars[$j]] = [$chars[$j], $chars[$i]];
        }

        return implode('', $chars);
    }
}
