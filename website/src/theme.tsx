/* eslint-disable react-refresh/only-export-components -- 主题 Provider 与 hook 同文件是约定俗成的组合 */
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { CssBaseline, ThemeProvider, createTheme, useMediaQuery } from '@mui/material'
import type { PaletteMode, Theme } from '@mui/material'

/** 图表与 UI 共用的品牌色板（按明暗模式区分） */
export const chartPalette = {
  dark: ['#4f8cff', '#22d3ee', '#34d399', '#fbbf24', '#f87171', '#a78bfa', '#f472b6', '#4ade80'],
  light: ['#2563eb', '#0891b2', '#059669', '#d97706', '#dc2626', '#7c3aed', '#db2777', '#16a34a'],
} as const

interface DesignTokens {
  palette: Record<string, unknown>
}

const getDesignTokens = (mode: PaletteMode): DesignTokens => ({
  palette: {
    mode,
    ...(mode === 'dark'
      ? {
          background: { default: '#0d1220', paper: '#151c2e' },
          primary: { main: '#4f8cff' },
          secondary: { main: '#22d3ee' },
          success: { main: '#34d399' },
          warning: { main: '#fbbf24' },
          error: { main: '#f87171' },
          divider: 'rgba(148, 163, 184, 0.16)',
        }
      : {
          background: { default: '#f3f5f9', paper: '#ffffff' },
          primary: { main: '#2563eb' },
          secondary: { main: '#0891b2' },
          success: { main: '#059669' },
          warning: { main: '#d97706' },
          error: { main: '#dc2626' },
          divider: 'rgba(15, 23, 42, 0.10)',
        }),
  },
})

export function buildTheme(mode: PaletteMode): Theme {
  const tokens = getDesignTokens(mode)
  return createTheme({
    ...tokens,
    shape: { borderRadius: 10 },
    typography: {
      fontFamily: '"Segoe UI", "PingFang SC", "Microsoft YaHei", Arial, sans-serif',
      h4: { fontWeight: 700 },
    },
    components: {
      MuiPaper: {
        styleOverrides: {
          root:
            mode === 'dark'
              ? { backgroundImage: 'none' }
              : {},
        },
      },
      MuiCard: { defaultProps: { elevation: 0, variant: 'outlined' } },
      MuiButton: { defaultProps: { disableElevation: true } },
      MuiTooltip: { defaultProps: { arrow: true } },
    },
  } as Parameters<typeof createTheme>[0])
}

export type ColorModePreference = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'nb.color-mode'

interface ColorModeContextValue {
  /** 当前生效的模式（system 时解析自系统偏好） */
  mode: PaletteMode
  /** 用户偏好设置 */
  preference: ColorModePreference
  setPreference: (preference: ColorModePreference) => void
  /** 在亮/暗之间切换（退出 system 跟随） */
  toggle: () => void
}

const ColorModeContext = createContext<ColorModeContextValue | null>(null)

export const useColorMode = (): ColorModeContextValue => {
  const ctx = useContext(ColorModeContext)
  if (!ctx) {
    throw new Error('useColorMode must be used within a ColorModeProvider')
  }
  return ctx
}

function readStoredPreference(): ColorModePreference {
  const stored = localStorage.getItem(STORAGE_KEY)
  // 暗色优先：未做过选择时默认深色（监控工具经典风格）
  return stored === 'light' || stored === 'dark' || stored === 'system' ? stored : 'dark'
}

export const ColorModeProvider = ({ children }: { children: ReactNode }) => {
  const [preference, setPreferenceState] = useState<ColorModePreference>(readStoredPreference)
  const prefersDark = useMediaQuery('(prefers-color-scheme: dark)', { noSsr: true })

  const mode: PaletteMode = preference === 'system' ? (prefersDark ? 'dark' : 'light') : preference
  const theme = useMemo(() => buildTheme(mode), [mode])

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, preference)
  }, [preference])

  const value = useMemo<ColorModeContextValue>(
    () => ({
      mode,
      preference,
      setPreference: setPreferenceState,
      toggle: () => setPreferenceState(mode === 'dark' ? 'light' : 'dark'),
    }),
    [mode, preference],
  )

  return (
    <ColorModeContext.Provider value={value}>
      <ThemeProvider theme={theme}>
        <CssBaseline enableColorScheme />
        {children}
      </ThemeProvider>
    </ColorModeContext.Provider>
  )
}
