import { createTheme } from '@mui/material/styles';

export const theme = createTheme({
  cssVariables: { colorSchemeSelector: 'class' },
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#5b5bd6' },
        background: { default: '#f7f7f9', paper: '#ffffff' },
      },
    },
    dark: {
      palette: {
        primary: { main: '#9b9bf4' },
        background: { default: '#111114', paper: '#18181c' },
      },
    },
  },
  shape: { borderRadius: 8 },
  typography: {
    fontFamily: 'Inter, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif',
    fontSize: 14,
  },
  components: {
    MuiButtonBase: { defaultProps: { disableRipple: true } },
    MuiChip: { defaultProps: { size: 'small' } },
    MuiTextField: { defaultProps: { size: 'small' } },
    MuiListItemButton: {
      styleOverrides: {
        root: ({ theme }) => ({
          '&.Mui-focusVisible, &.kala-focused': { outline: `2px solid ${theme.palette.primary.main}`, outlineOffset: -2 },
        }),
      },
    },
    MuiTooltip: { defaultProps: { enterDelay: 500 } },
  },
});
