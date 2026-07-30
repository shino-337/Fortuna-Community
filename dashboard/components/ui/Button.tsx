import React from 'react';
import { Loader2 } from 'lucide-react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg' | 'touch';
  isLoading?: boolean;
}

/**
 * A styled button component with variants and sizes.
 * @param children - Button label text or elements
 * @param variant - Visual style: primary (default), secondary, danger, ghost
 * @param size - Size: sm, md (default), lg
 * @param isLoading - Show loading spinner instead of children
 * @param className - Additional CSS classes
 */
export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(({
  children,
  variant = 'primary',
  size = 'md',
  isLoading,
  className = '',
  disabled,
  ...props
}, ref) => {
  const baseStyles = 'inline-flex min-w-0 items-center justify-center whitespace-nowrap rounded-lg font-semibold transition-colors duration-150 motion-reduce:transition-none focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-base disabled:opacity-50 disabled:cursor-not-allowed shadow-sm';

  const variants = {
    primary: 'bg-brand text-white hover:bg-brand-strong hover:shadow-brand/20 hover:shadow-md focus:ring-brand border border-transparent',
    secondary: 'bg-surface/80 text-text border border-border hover:bg-surface-2 hover:border-border/80 focus:ring-border',
    danger: 'bg-critical/10 text-critical border border-critical/30 hover:bg-critical/20 hover:border-critical/50 focus:ring-critical',
    ghost: 'bg-transparent text-muted hover:bg-surface hover:text-text focus:ring-border shadow-none',
  };

  const sizes = {
    sm: 'min-h-10 px-3 text-caption sm:h-8 sm:min-h-8',
    md: 'h-10 px-4 py-2 text-body',
    lg: 'h-12 px-6 text-base',
    touch: 'min-h-10 min-w-10 px-3 text-caption',
  };

  return (
    <button
      ref={ref}
      className={`${baseStyles} ${variants[variant]} ${sizes[size]} ${className}`}
      disabled={isLoading || disabled}
      {...props}
    >
      {isLoading && <Loader2 className="w-4 h-4 mr-2 animate-spin motion-reduce:animate-none" />}
      {children}
    </button>
  );
});

Button.displayName = 'Button';
