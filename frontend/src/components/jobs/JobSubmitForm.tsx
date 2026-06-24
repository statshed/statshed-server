/**
 * AIDEV-NOTE: Job submit form component
 * Dialog form for submitting job status updates
 */

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Button, Input, Select, Dialog } from '@/components/ui'
import { useSubmitStatus } from '@/hooks'
import type { JobStatus } from '@/types'

const statusOptions = [
  { value: 'success', label: 'Success' },
  { value: 'error', label: 'Error' },
  { value: 'progress', label: 'In Progress' },
]

// AIDEV-NOTE: Added max lengths and trim to prevent unbounded input
const jobSubmitSchema = z.object({
  group: z.string().trim().min(1, 'Group name is required').max(100, 'Group name too long'),
  job: z.string().trim().min(1, 'Job name is required').max(100, 'Job name too long'),
  status: z.enum(['success', 'error', 'progress'] as const),
  message: z.string().trim().max(1000, 'Message too long').optional(),
})

type FormData = z.infer<typeof jobSubmitSchema>

interface JobSubmitFormProps {
  isOpen: boolean
  onClose: () => void
  defaultGroup?: string
}

export default function JobSubmitForm({
  isOpen,
  onClose,
  defaultGroup = '',
}: JobSubmitFormProps) {
  const submitStatus = useSubmitStatus()

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormData>({
    resolver: zodResolver(jobSubmitSchema),
    defaultValues: {
      group: defaultGroup,
      job: '',
      status: 'success',
      message: '',
    },
  })

  // Reset form when dialog opens with default group
  useEffect(() => {
    if (isOpen) {
      reset({
        group: defaultGroup,
        job: '',
        status: 'success',
        message: '',
      })
    }
  }, [isOpen, defaultGroup, reset])

  const onSubmit = async (data: FormData) => {
    await submitStatus.mutateAsync({
      group: data.group,
      job: data.job,
      status: data.status as JobStatus,
      message: data.message || undefined,
    })
    onClose()
  }

  return (
    <Dialog isOpen={isOpen} onClose={onClose} title="Submit Job Status">
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <p className="text-sm text-gray-600 dark:text-gray-400">
          Submit a status update for a job. If the group or job doesn't exist, it will be created automatically.
        </p>

        <Input
          label="Group Name"
          placeholder="e.g., backups, deployments"
          error={errors.group?.message}
          {...register('group')}
        />

        <Input
          label="Job Name"
          placeholder="e.g., database-backup, api-deploy"
          error={errors.job?.message}
          {...register('job')}
        />

        <Select
          label="Status"
          options={statusOptions}
          error={errors.status?.message}
          {...register('status')}
        />

        <div className="space-y-1">
          <label
            htmlFor="message"
            className="block text-sm font-medium text-gray-700 dark:text-gray-300"
          >
            Message (optional)
          </label>
          <textarea
            id="message"
            rows={3}
            placeholder="Additional details about the job status..."
            className="block w-full rounded-lg border border-gray-300 dark:border-gray-600 px-3 py-2 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder:text-gray-400 dark:placeholder:text-gray-500 focus:outline-none focus:ring-2 focus:ring-primary-600 dark:focus:ring-primary-400 focus:border-transparent"
            {...register('message')}
          />
          {errors.message && (
            <p className="text-sm text-red-600 dark:text-red-400">
              {errors.message.message}
            </p>
          )}
        </div>

        <div className="flex justify-end gap-3 pt-4">
          <Button type="button" variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            isLoading={isSubmitting || submitStatus.isPending}
          >
            Submit Status
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
