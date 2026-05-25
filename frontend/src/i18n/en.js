export default {
  // Connection bar
  host: 'Host',
  user: 'User',
  password: 'Password',
  port: 'Port',
  connect: 'Connect',
  disconnect: 'Disconnect',
  connecting: 'Connecting...',
  protocol: 'Protocol',

  // Toolbar
  manageSites: 'Manage sites',
  settings: 'Settings',
  refresh: 'Refresh',
  newFolder: 'New folder',
  rename: 'Rename',
  delete: 'Delete',
  upload: 'Upload',
  download: 'Download',
  transfer: 'Transfer',
  queue: 'Queue',

  // File browser
  local: 'Local',
  remote: 'Remote',
  name: 'Name',
  size: 'Size',
  date: 'Date',
  type: 'Type',
  noConnection: 'No active connection',
  connectFirst: 'Connect to a server to browse files.',
  emptyFolder: 'Empty folder',
  parentFolder: '.. (parent)',

  // Transfer queue
  transferQueue: 'Transfer queue',
  pending: 'Queue',
  failed: 'Failed',
  success: 'Successful',
  cancel: 'Cancel',
  retry: 'Retry',
  clear: 'Clear',
  noTransfers: 'No transfers',

  // Site manager
  savedSites: 'Saved sites',
  newSite: 'New site',
  editSite: 'Edit site',
  siteName: 'Site name',
  host_label: 'Host',
  encryption: 'Encryption',
  authType: 'Authentication',
  sshKey: 'SSH key',
  browse: 'Browse',
  save: 'Save',
  close: 'Close',
  deleteSite: 'Delete site',
  confirmDelete: 'Are you sure you want to delete this site?',
  yes: 'Yes',
  no: 'No',
  remoteDir: 'Default remote directory',
  remoteDirHint: 'Directory to navigate to after connecting',

  // Auth types
  authAnonymous: 'Anonymous',
  authAskPassword: 'Ask password',
  authInteractive: 'Interactive',
  authNormal: 'Normal',
  authSSHKey: 'SSH Key',

  // Encryption types
  encNone: 'None',
  encTLS: 'TLS (Implicit)',
  encFTPES: 'FTPES (Explicit)',

  // Protocol tooltips
  ftpTooltip: 'FTP (File Transfer Protocol): standard insecure file transfer protocol. Fast and widely supported.',
  sftpTooltip: 'SFTP (SSH File Transfer Protocol): secure file transfer over SSH. Recommended for sensitive data.',

  // Settings
  settingsTitle: 'Settings',
  appearance: 'Appearance',
  theme: 'Theme',
  themeDark: 'Dark',
  themeLight: 'Light',
  language: 'Language',
  accentColor: 'Color',
  colorPickerTitle: 'Color picker',
  resetColor: 'Reset to default',
  colorHistory: 'Recent',
  transfers: 'Transfers',
  maxConcurrent: 'Max concurrent transfers',
  transferSpeedLimit: 'Speed limit (KB/s, 0 = unlimited)',
  avgSuffix: 'avg.',
  defaultLocalDir: 'Default local directory',
  connection: 'Connection',
  defaultPort: 'Default FTP port',
  timeout: 'Connection timeout (seconds)',
  passiveMode: 'Passive mode (PASV)',
  autoReconnect: 'Auto-reconnect',
  interface: 'Interface',
  showHiddenFiles: 'Show hidden files',
  confirmOnDelete: 'Confirm before deleting',
  dateFormat: 'Date format',
  saveSettings: 'Save',
  settingsSaved: 'Settings saved',

  // Site note
  siteNote: 'Note',
  copyNote: 'Copy note',
  copied: 'Copied!',

  // Ask password prompt
  passwordPromptTitle: 'Password required',
  passwordPromptLabel: 'Password for',
  paste: 'Paste',

  // Export / import
  exportSites: 'Export sites',
  importSites: 'Import sites',
  importedCount: 'sites imported',

  // Multi-connection
  maxConnections: 'Max simultaneous connections',
  keepOrReplaceTitle: 'Active connection',
  keepOrReplaceMsg: 'A connection is already active. What do you want to do?',
  keepConnection: 'Keep and open new',
  replaceConnection: 'Replace current connection',
  maxConnectionsReached: 'Connection limit reached',
  multiDisconnectTitle: 'Multiple active connections',
  multiDisconnectMsg: 'connections are open. Close all?',
  disconnectAll: 'Disconnect all',

  // Conflict resolution
  conflictTitle: 'File already exists',
  conflictReplace: 'Replace',
  conflictRenameHost: 'Rename on host',
  conflictRenameServer: 'Rename on server',
  conflictRenameTitle: 'New file name',
  conflictSkip: 'Skip',

  // Delete confirmation
  confirmDeleteFile: 'Delete this item?',
  deleteConfirm: 'Delete',
  items: 'items',

  // Errors
  connectError: 'Connection failed',
  unknownError: 'Unknown error',
}
