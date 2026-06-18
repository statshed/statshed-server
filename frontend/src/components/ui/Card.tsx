/**
 * AIDEV-NOTE: Card component
 * Compound component with Card, CardHeader, CardBody, CardFooter
 * Can be rendered as a link when 'to' prop is provided
 */

import { type ReactNode, type HTMLAttributes } from 'react'
import { Link, type LinkProps } from 'react-router-dom'
import { cn } from '@/lib/utils'
import type { HealthStatus } from '@/types'
import { HEALTH_STATUS_BG_COLORS } from '@/lib/constants'

interface CardBaseProps {
  children: ReactNode
  className?: string
  status?: HealthStatus
}

type CardAsDiv = CardBaseProps & HTMLAttributes<HTMLDivElement> & { to?: never }

type CardAsLink = CardBaseProps & Omit<LinkProps, 'className'> & { to: string }

type CardProps = CardAsDiv | CardAsLink

export function Card({ children, className, status, to, ...props }: CardProps) {
  const cardClasses = cn(
    'bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700',
    'shadow-sm transition-shadow',
    status && 'relative overflow-hidden',
    to && 'hover:shadow-md cursor-pointer',
    className
  )

  const content = (
    <>
      {status && (
        <div
          className={cn(
            'absolute top-0 left-0 right-0 h-1',
            HEALTH_STATUS_BG_COLORS[status]
          )}
        />
      )}
      {children}
    </>
  )

  if (to) {
    const linkProps = props as Omit<LinkProps, 'className' | 'to'>
    return (
      <Link to={to} className={cardClasses} {...linkProps}>
        {content}
      </Link>
    )
  }

  return (
    <div className={cardClasses} {...(props as HTMLAttributes<HTMLDivElement>)}>
      {content}
    </div>
  )
}

interface CardSectionProps {
  children: ReactNode
  className?: string
}

export function CardHeader({ children, className }: CardSectionProps) {
  return (
    <div
      className={cn(
        'px-4 py-3 border-b border-gray-200 dark:border-gray-700',
        className
      )}
    >
      {children}
    </div>
  )
}

export function CardBody({ children, className }: CardSectionProps) {
  return <div className={cn('p-4', className)}>{children}</div>
}

export function CardFooter({ children, className }: CardSectionProps) {
  return (
    <div
      className={cn(
        'px-4 py-3 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50',
        className
      )}
    >
      {children}
    </div>
  )
}
