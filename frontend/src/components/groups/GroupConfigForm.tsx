/**
 * AIDEV-NOTE: Group configuration form component
 * Form for editing group-specific timeout and expiration settings
 *
 * Configuration options:
 * - Progress timeout (minutes): how long before a 'progress' job times out
 * - Expiration (hours): how long before jobs are auto-deleted (required)
 * - Staleness (optional): enable warning state before expiration
 */

import { useEffect } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { AlertCircle, RefreshCw } from 'lucide-react'
import { Button, Input, Dialog, Checkbox } from '@/components/ui'
import { useGroupConfig, useUpdateGroupConfig } from '@/hooks'

// AIDEV-NOTE: Preprocess converts empty strings to null, then validates as positive integer
// This ensures invalid values like "abc" or "0" surface as form errors
const optionalPositiveInt = z.preprocess(
  (val) => (val === '' || val === null || val === undefined ? null : Number(val)),
  z.number().int().min(1, 'Must be at least 1').nullable()
)

// AIDEV-NOTE: Use refine for clearer error message when required field is empty
const requiredPositiveInt = z.preprocess(
  (val) => (val === '' || val === null || val === undefined ? null : Number(val)),
  z
    .number({ required_error: 'Expiration timeout is required' })
    .int('Must be a whole number')
    .min(1, 'Must be at least 1')
    .max(8760, 'Must be at most 8760 (1 year)')
    .nullable()
).refine((val) => val !== null, { message: 'Expiration timeout is required' })

// AIDEV-NOTE: Schema with cross-field validation for staleness < expiration
const groupConfigSchema = z
  .object({
    progress_timeout_minutes: optionalPositiveInt,
    expiration_timeout_hours: requiredPositiveInt,
    staleness_enabled: z.boolean(),
    staleness_timeout_hours: optionalPositiveInt,
  })
  .refine(
    (data) => {
      // If staleness is enabled, staleness_timeout_hours is required
      if (data.staleness_enabled && data.staleness_timeout_hours === null) {
        return false
      }
      return true
    },
    {
      message: 'Staleness timeout is required when staleness is enabled',
      path: ['staleness_timeout_hours'],
    }
  )
  .refine(
    (data) => {
      // staleness_timeout_hours must be less than expiration_timeout_hours
      if (
        data.staleness_enabled &&
        data.staleness_timeout_hours !== null &&
        data.expiration_timeout_hours !== null &&
        data.staleness_timeout_hours >= data.expiration_timeout_hours
      ) {
        return false
      }
      return true
    },
    {
      message: 'Staleness timeout must be less than expiration timeout',
      path: ['staleness_timeout_hours'],
    }
  )

type FormData = z.infer<typeof groupConfigSchema>

interface GroupConfigFormProps {
  groupName: string
  isOpen: boolean
  onClose: () => void
}

export default function GroupConfigForm({
  groupName,
  isOpen,
  onClose,
}: GroupConfigFormProps) {
  const { data: config, isLoading, isError, error, refetch } = useGroupConfig(groupName)
  const updateConfig = useUpdateGroupConfig(groupName)

  const {
    register,
    handleSubmit,
    reset,
    control,
    trigger,
    formState: { errors, isSubmitting, isSubmitted },
  } = useForm<FormData>({
    resolver: zodResolver(groupConfigSchema),
    defaultValues: {
      progress_timeout_minutes: null,
      expiration_timeout_hours: 24,
      staleness_enabled: false,
      staleness_timeout_hours: null,
    },
  })

  // Watch staleness_enabled to conditionally show/hide staleness timeout input
  const stalenessEnabled = useWatch({ control, name: 'staleness_enabled' })

  // AIDEV-NOTE: The "staleness must be less than expiration" error lives on the
  // staleness_timeout_hours path, but RHF only re-validates the field that changed.
  // So resolving the error by raising EXPIRATION wouldn't clear it until the next
  // submit. Re-validate the dependent staleness field whenever expiration changes
  // (only once the form has been submitted, matching the default onSubmit mode).
  const expirationValue = useWatch({ control, name: 'expiration_timeout_hours' })
  useEffect(() => {
    if (isSubmitted && stalenessEnabled) {
      void trigger('staleness_timeout_hours')
    }
  }, [expirationValue, isSubmitted, stalenessEnabled, trigger])

  // Reset form when config loads or dialog opens
  useEffect(() => {
    if (config && isOpen) {
      reset({
        progress_timeout_minutes: config.progress_timeout_minutes ?? null,
        expiration_timeout_hours: config.expiration_timeout_hours ?? 24,
        staleness_enabled: config.staleness_enabled ?? false,
        staleness_timeout_hours: config.staleness_timeout_hours ?? null,
      })
    }
  }, [config, isOpen, reset])

  const onSubmit = async (data: FormData) => {
    await updateConfig.mutateAsync({
      progress_timeout_minutes: data.progress_timeout_minutes,
      expiration_timeout_hours: data.expiration_timeout_hours,
      staleness_enabled: data.staleness_enabled,
      staleness_timeout_hours: data.staleness_enabled ? data.staleness_timeout_hours : null,
    })
    onClose()
  }

  // AIDEV-NOTE: Guard the read error before rendering the form. Without this, a failed
  // GET /config left the form showing hardcoded defaults (24h expiration) with Save enabled,
  // so a Save would overwrite the real server config with those defaults.
  if (isError) {
    return (
      <Dialog isOpen={isOpen} onClose={onClose} title={`Configure ${groupName}`}>
        <div className="space-y-4">
          <div className="flex items-start gap-3 rounded-lg bg-red-50 dark:bg-red-900/20 p-4">
            <AlertCircle className="w-5 h-5 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-gray-900 dark:text-white">
                Failed to load group configuration
              </p>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
                {error?.message || 'An error occurred while loading configuration.'}
              </p>
            </div>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button type="button" variant="secondary" onClick={onClose}>
              Close
            </Button>
            <Button type="button" onClick={() => refetch()}>
              <RefreshCw className="w-4 h-4" />
              Try Again
            </Button>
          </div>
        </div>
      </Dialog>
    )
  }

  return (
    <Dialog isOpen={isOpen} onClose={onClose} title={`Configure ${groupName}`}>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-6">
        <p className="text-sm text-gray-600 dark:text-gray-400">
          Configure timeout and expiration settings for this group.
        </p>

        {/* Effective values summary */}
        {config && (
          <div className="text-xs text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800/50 rounded-lg p-3">
            <p className="font-medium mb-1">Current effective values:</p>
            <ul className="list-disc list-inside space-y-0.5">
              <li>Progress timeout: {config.effective_progress_timeout_minutes} minutes</li>
              <li>Expiration: {config.effective_expiration_timeout_hours} hours</li>
              <li>
                Staleness:{' '}
                {config.staleness_enabled && config.effective_staleness_timeout_hours !== null
                  ? `${config.effective_staleness_timeout_hours} hours`
                  : 'disabled'}
              </li>
            </ul>
          </div>
        )}

        {/* Progress Timeout */}
        <Input
          label="Progress Timeout (minutes)"
          type="number"
          min="1"
          placeholder={
            config
              ? `Default: ${config.effective_progress_timeout_minutes}`
              : 'Loading...'
          }
          helperText="Time before a 'progress' job times out"
          error={errors.progress_timeout_minutes?.message}
          disabled={isLoading}
          {...register('progress_timeout_minutes')}
        />

        {/* Expiration Section */}
        <div className="space-y-2 p-4 border border-gray-200 dark:border-gray-700 rounded-lg">
          <h4 className="text-sm font-medium text-gray-900 dark:text-white">
            Expiration
          </h4>
          <Input
            label="Expiration Timeout (hours)"
            type="number"
            min="1"
            max="8760"
            placeholder="24"
            helperText="Jobs auto-delete after this time without updates"
            error={errors.expiration_timeout_hours?.message}
            disabled={isLoading}
            {...register('expiration_timeout_hours')}
          />
        </div>

        {/* Staleness Section */}
        <div className="space-y-3 p-4 border border-gray-200 dark:border-gray-700 rounded-lg">
          <h4 className="text-sm font-medium text-gray-900 dark:text-white">
            Staleness Warning
          </h4>
          <Checkbox
            label="Enable staleness warnings"
            helperText="Jobs show a warning state before they expire"
            disabled={isLoading}
            {...register('staleness_enabled')}
          />
          {stalenessEnabled && (
            <Input
              label="Staleness Timeout (hours)"
              type="number"
              min="1"
              placeholder={
                config && config.effective_staleness_timeout_hours !== null
                  ? `Default: ${config.effective_staleness_timeout_hours}`
                  : 'Enter hours'
              }
              helperText="Must be less than expiration timeout"
              error={errors.staleness_timeout_hours?.message}
              disabled={isLoading}
              {...register('staleness_timeout_hours')}
            />
          )}
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            isLoading={isSubmitting || updateConfig.isPending}
            disabled={isLoading}
          >
            Save Changes
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
