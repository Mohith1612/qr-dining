"use client"

import { Component, ReactNode } from "react"
import { Button } from "@/components/ui/button"

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  reset = () => this.setState({ hasError: false })

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div
          className="flex flex-col items-center justify-center min-h-[40vh] px-6 text-center gap-4"
          style={{ color: "var(--color-text)" }}
        >
          <p className="text-sm" style={{ color: "var(--color-text-muted)" }}>
            Something went wrong
          </p>
          <Button variant="outline" onClick={this.reset}>
            Try again
          </Button>
        </div>
      )
    }
    return this.props.children
  }
}
