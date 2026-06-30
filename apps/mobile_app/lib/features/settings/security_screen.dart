import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class SecurityScreen extends StatefulWidget {
  const SecurityScreen({super.key});

  @override
  State<SecurityScreen> createState() => _SecurityScreenState();
}

class _SecurityScreenState extends State<SecurityScreen> {
  bool _twoFactor = false;
  bool _loginAlerts = true;
  bool _saveLoginInfo = true;
  bool _faceId = false;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0A0A0A),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0A0A0A),
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new,
              color: Colors.white, size: 18),
          onPressed: () => context.pop(),
        ),
        title: const Text('Security',
            style: TextStyle(
                color: Colors.white, fontWeight: FontWeight.w700)),
        centerTitle: true,
      ),
      body: ListView(
        children: [
          // ── Login & Password ─────────────────────────────────────────────
          _Header('Login & Password'),
          _ActionTile(
            icon: Icons.lock_outline,
            title: 'Change password',
            onTap: () => _showChangePassword(context),
          ),
          _ActionTile(
            icon: Icons.phone_outlined,
            title: 'Phone number',
            subtitle: '+1 *** *** 4567',
            onTap: () => _showEditField(context, 'Phone number'),
          ),
          _ActionTile(
            icon: Icons.email_outlined,
            title: 'Email address',
            subtitle: 'a***@gmail.com',
            onTap: () => _showEditField(context, 'Email address'),
          ),
          _ActionTile(
            icon: Icons.alternate_email,
            title: 'Username',
            subtitle: '@alexjohnson',
            onTap: () => _showEditField(context, 'Username'),
          ),

          // ── Security ─────────────────────────────────────────────────────
          _Header('Security'),
          _SwitchTile(
            icon: Icons.verified_user_outlined,
            title: 'Two-factor authentication',
            subtitle: 'Add an extra layer of security to your account',
            value: _twoFactor,
            onChanged: (v) {
              setState(() => _twoFactor = v);
              if (v) _show2FASheet(context);
            },
          ),
          _SwitchTile(
            icon: Icons.notifications_active_outlined,
            title: 'Login alerts',
            subtitle: 'Get notified of new logins to your account',
            value: _loginAlerts,
            onChanged: (v) => setState(() => _loginAlerts = v),
          ),
          _SwitchTile(
            icon: Icons.save_outlined,
            title: 'Save login info',
            subtitle: 'Stay logged in on this device',
            value: _saveLoginInfo,
            onChanged: (v) => setState(() => _saveLoginInfo = v),
          ),
          _SwitchTile(
            icon: Icons.face_outlined,
            title: 'Face ID / Fingerprint',
            subtitle: 'Use biometrics to unlock the app',
            value: _faceId,
            onChanged: (v) => setState(() => _faceId = v),
          ),

          // ── Connected Apps ───────────────────────────────────────────────
          _Header('Connected Apps'),
          _ActionTile(
            icon: Icons.g_mobiledata,
            title: 'Google',
            subtitle: 'Connected',
            trailing: _Chip('Connected', const Color(0xFF4CAF50)),
            onTap: () => _showDisconnectDialog(context, 'Google'),
          ),
          _ActionTile(
            icon: Icons.apple,
            title: 'Apple',
            subtitle: 'Not connected',
            trailing: _Chip('Connect', Colors.white24),
            onTap: () => _snack(context, 'Apple sign-in — coming soon'),
          ),
          _ActionTile(
            icon: Icons.facebook_outlined,
            title: 'Facebook',
            subtitle: 'Not connected',
            trailing: _Chip('Connect', Colors.white24),
            onTap: () => _snack(context, 'Facebook sign-in — coming soon'),
          ),

          // ── Active Sessions ──────────────────────────────────────────────
          _Header('Active Sessions'),
          _ActionTile(
            icon: Icons.phone_android,
            title: 'This device',
            subtitle: 'Android · Active now',
            onTap: () {},
          ),
          _ActionTile(
            icon: Icons.devices_other,
            title: 'Manage all devices',
            onTap: () => _showDevicesSheet(context),
          ),

          // ── Danger Zone ──────────────────────────────────────────────────
          _Header('Account'),
          _ActionTile(
            icon: Icons.pause_circle_outline,
            title: 'Deactivate account',
            subtitle: 'Temporarily disable your account',
            onTap: () => _showDeactivateDialog(context),
          ),
          _ActionTile(
            icon: Icons.delete_forever_outlined,
            title: 'Delete account',
            subtitle: 'Permanently delete your account and data',
            titleColor: const Color(0xFFEE1D52),
            onTap: () => _showDeleteDialog(context),
          ),

          const SizedBox(height: 32),
        ],
      ),
    );
  }

  void _snack(BuildContext context, String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: const Color(0xFF1A1A1A),
      behavior: SnackBarBehavior.floating,
    ));
  }

  void _showChangePassword(BuildContext context) {
    final current = TextEditingController();
    final newPw = TextEditingController();
    final confirm = TextEditingController();
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(
            20, 20, 20, MediaQuery.of(ctx).viewInsets.bottom + 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Change Password',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.bold)),
            const SizedBox(height: 16),
            _PwField(controller: current, label: 'Current password'),
            const SizedBox(height: 12),
            _PwField(controller: newPw, label: 'New password'),
            const SizedBox(height: 12),
            _PwField(
                controller: confirm, label: 'Confirm new password'),
            const SizedBox(height: 8),
            const Text(
              'Password must be at least 8 characters with letters and numbers.',
              style: TextStyle(color: Colors.white38, fontSize: 12),
            ),
            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () {
                  if (newPw.text != confirm.text) {
                    _snack(context, 'Passwords do not match');
                    return;
                  }
                  if (newPw.text.length < 8) {
                    _snack(context,
                        'Password must be at least 8 characters');
                    return;
                  }
                  Navigator.pop(ctx);
                  _snack(context, 'Password updated successfully');
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFEE1D52),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
                child: const Text('Update password',
                    style: TextStyle(color: Colors.white)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showEditField(BuildContext context, String field) {
    final ctrl = TextEditingController();
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(
            20, 20, 20, MediaQuery.of(ctx).viewInsets.bottom + 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Change $field',
                style: const TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.bold)),
            const SizedBox(height: 16),
            TextField(
              controller: ctrl,
              autofocus: true,
              style: const TextStyle(color: Colors.white),
              decoration: InputDecoration(
                hintText: 'Enter new $field',
                hintStyle: const TextStyle(color: Colors.white38),
                filled: true,
                fillColor: Colors.white10,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () {
                  Navigator.pop(ctx);
                  _snack(context, '$field updated');
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFEE1D52),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
                child: const Text('Save',
                    style: TextStyle(color: Colors.white)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _show2FASheet(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: SingleChildScrollView(
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('Two-Factor Authentication',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 18,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 12),
              const Text(
                'Choose a second verification method:',
                style: TextStyle(color: Colors.white54),
              ),
              const SizedBox(height: 16),
              ListTile(
                leading: const Icon(Icons.sms_outlined,
                    color: Colors.white70),
                title: const Text('SMS',
                    style: TextStyle(color: Colors.white)),
                subtitle: const Text('+1 *** *** 4567',
                    style: TextStyle(color: Colors.white38)),
                onTap: () {
                  Navigator.pop(context);
                  _snack(context, '2FA via SMS enabled');
                },
              ),
              ListTile(
                leading: const Icon(Icons.email_outlined,
                    color: Colors.white70),
                title: const Text('Email',
                    style: TextStyle(color: Colors.white)),
                subtitle: const Text('a***@gmail.com',
                    style: TextStyle(color: Colors.white38)),
                onTap: () {
                  Navigator.pop(context);
                  _snack(context, '2FA via email enabled');
                },
              ),
              ListTile(
                leading: const Icon(Icons.smartphone_outlined,
                    color: Colors.white70),
                title: const Text('Authenticator app',
                    style: TextStyle(color: Colors.white)),
                onTap: () {
                  Navigator.pop(context);
                  _snack(context, 'Authenticator app — coming soon');
                },
              ),
            ],
            ),
          ),
        ),
      ),
    );
  }

  void _showDisconnectDialog(BuildContext context, String app) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: Text('Disconnect $app',
            style: const TextStyle(color: Colors.white)),
        content: Text(
          'Are you sure you want to disconnect your $app account?',
          style: const TextStyle(color: Colors.white70),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel',
                style: TextStyle(color: Colors.white54)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              _snack(context, '$app disconnected');
            },
            child: const Text('Disconnect',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }

  void _showDevicesSheet(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
            const SizedBox(height: 16),
            const Text('Active Devices',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            ListTile(
              leading: const Icon(Icons.phone_android,
                  color: Colors.white70),
              title: const Text('Android · This device',
                  style: TextStyle(color: Colors.white)),
              subtitle: const Text('Active now',
                  style: TextStyle(
                      color: Color(0xFF4CAF50), fontSize: 12)),
            ),
            ListTile(
              leading:
                  const Icon(Icons.laptop, color: Colors.white70),
              title: const Text('Chrome · Windows',
                  style: TextStyle(color: Colors.white)),
              subtitle: const Text('2 days ago',
                  style: TextStyle(
                      color: Colors.white38, fontSize: 12)),
              trailing: TextButton(
                onPressed: () {
                  Navigator.pop(context);
                  _snack(context, 'Session logged out');
                },
                child: const Text('Log out',
                    style: TextStyle(
                        color: Color(0xFFEE1D52), fontSize: 12)),
              ),
            ),
            const SizedBox(height: 16),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: SizedBox(
                width: double.infinity,
                child: OutlinedButton(
                  onPressed: () {
                    Navigator.pop(context);
                    _snack(context,
                        'All other devices logged out');
                  },
                  style: OutlinedButton.styleFrom(
                    foregroundColor: const Color(0xFFEE1D52),
                    side: const BorderSide(
                        color: Color(0xFFEE1D52)),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8)),
                  ),
                  child: const Text('Log out all other devices'),
                ),
              ),
            ),
            const SizedBox(height: 16),
          ],
          ),
        ),
      ),
    );
  }

  void _showDeactivateDialog(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Deactivate account',
            style: TextStyle(color: Colors.white)),
        content: const Text(
          'Your account will be hidden until you log back in. Your videos and profile will not be visible to others.',
          style: TextStyle(color: Colors.white70),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel',
                style: TextStyle(color: Colors.white54)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              _snack(context, 'Account deactivated');
            },
            child: const Text('Deactivate',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }

  void _showDeleteDialog(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Delete account',
            style: TextStyle(color: Colors.white)),
        content: const Text(
          'This is permanent and cannot be undone. All your videos, followers, messages and data will be deleted forever.',
          style: TextStyle(color: Colors.white70),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel',
                style: TextStyle(color: Colors.white54)),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Delete forever',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }
}

// ── Shared widgets ────────────────────────────────────────────────────────────

class _Header extends StatelessWidget {
  final String title;
  const _Header(this.title);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 20, 16, 6),
      child: Text(
        title.toUpperCase(),
        style: const TextStyle(
          color: Colors.white38,
          fontSize: 11,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.8,
        ),
      ),
    );
  }
}

class _ActionTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback onTap;
  final Color? titleColor;
  final Widget? trailing;

  const _ActionTile({
    required this.icon,
    required this.title,
    required this.onTap,
    this.subtitle,
    this.titleColor,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(icon, color: Colors.white70, size: 22),
      title: Text(title,
          style: TextStyle(
              color: titleColor ?? Colors.white, fontSize: 15)),
      subtitle: subtitle != null
          ? Text(subtitle!,
              style: const TextStyle(
                  color: Colors.white38, fontSize: 12))
          : null,
      trailing: trailing ??
          const Icon(Icons.chevron_right,
              color: Colors.white24, size: 20),
      onTap: onTap,
    );
  }
}

class _SwitchTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _SwitchTile({
    required this.icon,
    required this.title,
    required this.value,
    required this.onChanged,
    this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      secondary: Icon(icon, color: Colors.white70, size: 22),
      title: Text(title,
          style: const TextStyle(color: Colors.white, fontSize: 15)),
      subtitle: subtitle != null
          ? Text(subtitle!,
              style: const TextStyle(
                  color: Colors.white38, fontSize: 12))
          : null,
      value: value,
      onChanged: onChanged,
      activeTrackColor: const Color(0xFFEE1D52),
    );
  }
}

class _Chip extends StatelessWidget {
  final String label;
  final Color color;
  const _Chip(this.label, this.color);

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color.withValues(alpha: 0.4)),
      ),
      child: Text(label,
          style: TextStyle(
              color: color, fontSize: 11, fontWeight: FontWeight.w600)),
    );
  }
}

class _PwField extends StatefulWidget {
  final TextEditingController controller;
  final String label;
  const _PwField({required this.controller, required this.label});

  @override
  State<_PwField> createState() => _PwFieldState();
}

class _PwFieldState extends State<_PwField> {
  bool _obscure = true;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: widget.controller,
      obscureText: _obscure,
      style: const TextStyle(color: Colors.white),
      decoration: InputDecoration(
        labelText: widget.label,
        labelStyle: const TextStyle(color: Colors.white38),
        filled: true,
        fillColor: Colors.white10,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: BorderSide.none,
        ),
        suffixIcon: IconButton(
          icon: Icon(
            _obscure ? Icons.visibility_off : Icons.visibility,
            color: Colors.white38,
          ),
          onPressed: () => setState(() => _obscure = !_obscure),
        ),
      ),
    );
  }
}