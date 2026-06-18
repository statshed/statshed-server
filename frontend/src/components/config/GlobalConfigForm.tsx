/**
 * AIDEV-NOTE: Global configuration form component
 * Form for editing global timeout settings
 */

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { AlertCircle, RefreshCw } from 'lucide-react'
import { Button, Input, Card, CardHeader, CardBody } from '@/components/ui'
import { useConfig, useUpdateConfig } from '@/hooks'

// Input schema for form fields
// AIDEV-NOTE: Validate timeouts as positive whole numbers in the SCHEMA so zodResolver
// surfaces per-field errors and blocks submit. Previously these were only checked as
// non-empty strings, with the real numeric checks in a try/catch that swallowed errors —
// so 0/-3 silently did nothing and 1.5 was truncated to 1 and saved with a success toast.
const positiveIntField = z
  .string()
  .trim()
  .min(1, 'Required')
  .refine((v) => /^\d+$/.test(v) && parseInt(v, 10) >= 1, 'Must be a whole number of at least 1')

// AIDEV-NOTE: Exported for unit testing the validation rules directly.
export const globalConfigInputSchema = z.object({
  progress_timeout_minutes: positiveIntField,
  staleness_timeout_hours: positiveIntField,
})

type FormInputData = z.infer<typeof globalConfigInputSchema>

// Parse and validate form data
function parseFormData(data: FormInputData) {
  const progress = parseInt(data.progress_timeout_minutes, 10)
  const staleness = parseInt(data.staleness_timeout_hours, 10)

  if (isNaN(progress) || progress < 1) {
    throw new Error('Progress timeout must be a positive number')
  }
  if (isNaN(staleness) || staleness < 1) {
    throw new Error('Staleness timeout must be a positive number')
  }

  return {
    progress_timeout_minutes: progress,
    staleness_timeout_hours: staleness,
  }
}

export default function GlobalConfigForm() {
  const { data: config, isLoading, isError, error, refetch } = useConfig()
  const updateConfig = useUpdateConfig()

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<FormInputData>({
    resolver: zodResolver(globalConfigInputSchema),
    defaultValues: {
      progress_timeout_minutes: '',
      staleness_timeout_hours: '',
    },
  })

  // Populate form when config loads
  useEffect(() => {
    if (config) {
      reset({
        progress_timeout_minutes: config.progress_timeout_minutes.toString(),
        staleness_timeout_hours: config.staleness_timeout_hours.toString(),
      })
    }
  }, [config, reset])

  const onSubmit = async (data: FormInputData) => {
    try {
      const parsed = parseFormData(data)
      await updateConfig.mutateAsync(parsed)
    } catch (err) {
      // Error will be handled by mutation's onError
      console.error('Failed to parse form data:', err)
    }
  }

  // AIDEV-NOTE: Guard the read error before rendering the form. Without this the form
  // renders with blank defaults on a failed GET /config — invisible to the user — and a
  // submit would PUT those blanks over the real server config. Mirrors the error-card
  // pattern in Dashboard/GroupDetail (AlertCircle + Try Again -> refetch).
  if (isError) {
    return (
      <Card>
        <CardBody>
          <div className="flex flex-col items-center justify-center py-12 text-center">
            <div className="p-4 bg-red-100 dark:bg-red-900/30 rounded-full mb-4">
              <AlertCircle className="w-8 h-8 text-red-600 dark:text-red-400" />
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-1">
              Failed to load settings
            </h3>
            <p className="text-gray-500 dark:text-gray-400 max-w-sm mb-4">
              {error?.message || 'An error occurred while loading configuration.'}
            </p>
            <Button onClick={() => refetch()} variant="secondary">
              <RefreshCw className="w-4 h-4" />
              Try Again
            </Button>
          </div>
        </CardBody>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
          Timeout Settings
        </h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          Configure default timeout values for all groups. Individual groups can override these settings.
        </p>
      </CardHeader>
      <CardBody>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <Input
            label="Progress Timeout (minutes)"
            type="number"
            min="1"
            helperText="Jobs in 'progress' status will be marked as 'timeout' after this duration."
            error={errors.progress_timeout_minutes?.message}
            disabled={isLoading}
            {...register('progress_timeout_minutes')}
          />

          <Input
            label="Staleness Timeout (hours)"
            type="number"
            min="1"
            helperText="Successful jobs will be marked as 'stale' if not updated within this duration."
            error={errors.staleness_timeout_hours?.message}
            disabled={isLoading}
            {...register('staleness_timeout_hours')}
          />

          <div className="flex justify-end pt-4">
            <Button
              type="submit"
              isLoading={isSubmitting || updateConfig.isPending}
              disabled={isLoading || !isDirty}
            >
              Save Settings
            </Button>
          </div>
        </form>
      </CardBody>
    </Card>
  )
}
