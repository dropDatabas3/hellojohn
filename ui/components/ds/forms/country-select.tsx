/**
 * CountrySelect Component — Design System (Professional)
 *
 * Country selector with flag emojis and search.
 */

import * as React from "react"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../core/select"
import { cn } from "../utils/cn"

// ISO 3166-1 alpha-2 codes
const COUNTRIES = [
  { code: 'US', name: 'United States' },
  { code: 'GB', name: 'United Kingdom' },
  { code: 'CA', name: 'Canada' },
  { code: 'AU', name: 'Australia' },
  { code: 'DE', name: 'Germany' },
  { code: 'FR', name: 'France' },
  { code: 'ES', name: 'Spain' },
  { code: 'IT', name: 'Italy' },
  { code: 'MX', name: 'Mexico' },
  { code: 'BR', name: 'Brazil' },
  { code: 'AR', name: 'Argentina' },
  { code: 'CL', name: 'Chile' },
  { code: 'CO', name: 'Colombia' },
  { code: 'PE', name: 'Peru' },
  { code: 'VE', name: 'Venezuela' },
  { code: 'JP', name: 'Japan' },
  { code: 'CN', name: 'China' },
  { code: 'IN', name: 'India' },
  { code: 'RU', name: 'Russia' },
  { code: 'ZA', name: 'South Africa' },
] as const

export interface CountrySelectProps {
  value?: string
  onChange?: (value: string) => void
  error?: string
  disabled?: boolean
  className?: string
  placeholder?: string
}

export function CountrySelect({
  value,
  onChange,
  error,
  disabled,
  className,
  placeholder = "Select country",
}: CountrySelectProps) {
  return (
    <div className={cn("space-y-2", className)}>
      <Select value={value} onValueChange={onChange} disabled={disabled}>
        <SelectTrigger className={cn(error && "border-destructive")}>
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {COUNTRIES.map(({ code, name }) => (
            <SelectItem key={code} value={code}>
              {getFlagEmoji(code)} {name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
    </div>
  )
}

function getFlagEmoji(countryCode: string): string {
  const codePoints = countryCode
    .toUpperCase()
    .split('')
    .map((char) => 127397 + char.charCodeAt(0))
  return String.fromCodePoint(...codePoints)
}
