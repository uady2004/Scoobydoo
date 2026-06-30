import 'dart:async';
import 'dart:io';
import 'package:go_router/go_router.dart';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/update_profile_usecase.dart';
import 'package:tiktok_clone/features/profile/presentation/providers/profile_provider.dart';

enum _UsernameStatus { idle, checking, available, taken, error }

class EditProfileScreen extends ConsumerStatefulWidget {
  const EditProfileScreen({super.key, this.profile});

  final ProfileEntity? profile;

  @override
  ConsumerState<EditProfileScreen> createState() =>
      _EditProfileScreenState();
}

class _EditProfileScreenState extends ConsumerState<EditProfileScreen> {
  final _formKey = GlobalKey<FormState>();
  final _picker = ImagePicker();

  late final TextEditingController _nameCtrl;
  late final TextEditingController _usernameCtrl;
  late final TextEditingController _bioCtrl;
  late final TextEditingController _websiteCtrl;

  File? _pendingAvatarFile;
  bool _isPrivate = false;
  bool _controllersInitialized = false;

  ProfileEntity? _resolvedProfile;

  _UsernameStatus _usernameStatus = _UsernameStatus.idle;
  Timer? _debounce;
  String? _lastCheckedUsername;

  @override
  void initState() {
    super.initState();
    _nameCtrl = TextEditingController();
    _usernameCtrl = TextEditingController();
    _bioCtrl = TextEditingController();
    _websiteCtrl = TextEditingController();
    if (widget.profile != null) {
      _initControllers(widget.profile!);
    }
  }

  void _initControllers(ProfileEntity p) {
    if (_controllersInitialized) return;
    _resolvedProfile = p;
    _nameCtrl.text = p.displayName;
    _usernameCtrl.text = p.username;
    _bioCtrl.text = p.bio ?? '';
    _websiteCtrl.text = p.website ?? '';
    _isPrivate = p.isPrivate;
    _controllersInitialized = true;
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _nameCtrl.dispose();
    _usernameCtrl.dispose();
    _bioCtrl.dispose();
    _websiteCtrl.dispose();
    super.dispose();
  }

  // ── Avatar selection ───────────────────────────────────────────────────────

  Future<void> _pickAvatar() async {
    final source = await showModalBottomSheet<ImageSource>(
      context: context,
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.camera_alt_outlined),
              title: const Text('Camera'),
              onTap: () =>
                  Navigator.pop(context, ImageSource.camera), // ← keep Navigator.pop here — returns value from sheet
            ),
            ListTile(
              leading: const Icon(Icons.photo_library_outlined),
              title: const Text('Choose from gallery'),
              onTap: () =>
                  Navigator.pop(context, ImageSource.gallery), // ← keep Navigator.pop here — returns value from sheet
            ),
          ],
        ),
      ),
    );
    if (source == null) return;

    final picked = await _picker.pickImage(
      source: source,
      maxWidth: 512,
      maxHeight: 512,
      imageQuality: 88,
    );
    if (picked == null) return;

    setState(() => _pendingAvatarFile = File(picked.path));
  }

  // ── Username availability ──────────────────────────────────────────────────

  void _onUsernameChanged(String value) {
    _debounce?.cancel();
    final trimmed = value.trim();

    if (trimmed == _resolvedProfile?.username) {
      setState(() => _usernameStatus = _UsernameStatus.idle);
      return;
    }
    if (trimmed.isEmpty || trimmed.length < 3) {
      setState(() => _usernameStatus = _UsernameStatus.idle);
      return;
    }

    setState(() => _usernameStatus = _UsernameStatus.checking);

    _debounce = Timer(const Duration(milliseconds: 600), () async {
      if (trimmed == _lastCheckedUsername) return;
      _lastCheckedUsername = trimmed;
      try {
        final response = await ApiClient.instance.dio.get<Map<String, dynamic>>(
          '/users/check-username',
          queryParameters: {'username': trimmed},
        );
        final available =
            response.data?['available'] as bool? ?? false;
        if (mounted && _usernameCtrl.text.trim() == trimmed) {
          setState(() => _usernameStatus = available
              ? _UsernameStatus.available
              : _UsernameStatus.taken);
        }
      } catch (_) {
        if (mounted) {
          setState(() => _usernameStatus = _UsernameStatus.error);
        }
      }
    });
  }

  // ── Save ───────────────────────────────────────────────────────────────────

  Future<void> _save() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    if (_usernameStatus == _UsernameStatus.taken) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text('That username is taken. Choose another.')),
      );
      return;
    }

    final params = UpdateProfileParams(
      displayName: _nameCtrl.text.trim(),
      username: _usernameCtrl.text.trim(),
      bio: _bioCtrl.text.trim(),
      website: _websiteCtrl.text.trim(),
      isPrivate: _isPrivate,
    );

    await ref.read(editProfileProvider.notifier).save(
          params: params,
          newAvatarFile: _pendingAvatarFile,
          userId: _resolvedProfile?.userId ?? '',
        );

    final result = ref.read(editProfileProvider).valueOrNull;
    if (!mounted) return;

    if (result?.savedSuccessfully == true) {
      ref.read(editProfileProvider.notifier).resetSaveSuccess();
      context.pop(); // ← CHANGED from Navigator.pop(context)
    } else if (result?.error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result!.error!)),
      );
    }
  }

  // ── Widgets ────────────────────────────────────────────────────────────────

  Widget _buildAvatarSection() {
    final currentUrl = _resolvedProfile?.avatarUrl;

    return GestureDetector(
      onTap: _pickAvatar,
      child: Stack(
        alignment: Alignment.bottomRight,
        children: [
          CircleAvatar(
            radius: 44,
            backgroundColor: Colors.grey.shade800,
            backgroundImage: _pendingAvatarFile != null
                ? FileImage(_pendingAvatarFile!) as ImageProvider
                : (currentUrl != null
                    ? CachedNetworkImageProvider(currentUrl)
                    : null),
            child: _pendingAvatarFile == null && currentUrl == null
                ? const Icon(Icons.person_rounded,
                    size: 44, color: Colors.white38)
                : null,
          ),
          Container(
            padding: const EdgeInsets.all(6),
            decoration: const BoxDecoration(
              color: Color(0xFFEE1D52),
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.edit_rounded,
                color: Colors.white, size: 14),
          ),
        ],
      ),
    );
  }

  Widget _buildUsernameTrailing() {
    return switch (_usernameStatus) {
      _UsernameStatus.checking => const SizedBox(
          width: 18,
          height: 18,
          child: CircularProgressIndicator(strokeWidth: 2)),
      _UsernameStatus.available =>
        const Icon(Icons.check_circle_rounded,
            color: Color(0xFF4CAF50), size: 20),
      _UsernameStatus.taken =>
        const Icon(Icons.cancel_rounded,
            color: Color(0xFFEE1D52), size: 20),
      _UsernameStatus.error =>
        const Icon(Icons.error_outline_rounded,
            color: Colors.orange, size: 20),
      _UsernameStatus.idle => const SizedBox.shrink(),
    };
  }

  // ── Main build ─────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    if (!_controllersInitialized) {
      final ownAsync = ref.watch(ownProfileProvider);
      return ownAsync.when(
        loading: () => const Scaffold(
          backgroundColor: Colors.black,
          body: Center(
              child: CircularProgressIndicator(
                  color: Color(0xFFEE1D52))),
        ),
        error: (e, _) => Scaffold(
          backgroundColor: Colors.black,
          body: Center(
              child: Text('Error: $e',
                  style: const TextStyle(color: Colors.white))),
        ),
        data: (p) {
          if (p != null) _initControllers(p);
          if (!_controllersInitialized) {
            return const Scaffold(
              backgroundColor: Colors.black,
              body: Center(
                  child: Text('Not signed in',
                      style: TextStyle(color: Colors.white))),
            );
          }
          return _buildForm(context);
        },
      );
    }

    return _buildForm(context);
  }

  Widget _buildForm(BuildContext context) {
    final editState = ref.watch(editProfileProvider).valueOrNull;
    final isSaving = editState?.isSaving ?? false;

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.close_rounded, color: Colors.white),
          onPressed:
              isSaving ? null : () => context.pop(), // ← CHANGED
        ),
        title: const Text(
          'Edit profile',
          style: TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w700),
        ),
        centerTitle: true,
        actions: [
          TextButton(
            onPressed: isSaving ? null : _save,
            child: isSaving
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: Color(0xFFEE1D52)),
                  )
                : const Text(
                    'Save',
                    style: TextStyle(
                      color: Color(0xFFEE1D52),
                      fontWeight: FontWeight.w700,
                      fontSize: 15,
                    ),
                  ),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.symmetric(horizontal: 20),
          children: [
            const SizedBox(height: 24),
            Center(child: _buildAvatarSection()),
            const SizedBox(height: 8),
            Center(
              child: TextButton(
                onPressed: _pickAvatar,
                child: const Text(
                  'Change photo',
                  style: TextStyle(
                      color: Color(0xFF20D5EC),
                      fontWeight: FontWeight.w600),
                ),
              ),
            ),
            const SizedBox(height: 16),

            _EditField(
              label: 'Name',
              controller: _nameCtrl,
              maxLength: 30,
              validator: (v) {
                if (v == null || v.trim().isEmpty) {
                  return 'Name cannot be empty';
                }
                return null;
              },
            ),
            const SizedBox(height: 16),

            _EditField(
              label: 'Username',
              controller: _usernameCtrl,
              prefix: const Text('@',
                  style: TextStyle(color: Colors.white54)),
              trailing: _buildUsernameTrailing(),
              onChanged: _onUsernameChanged,
              validator: (v) {
                final trimmed = v?.trim() ?? '';
                if (trimmed.isEmpty) return 'Username cannot be empty';
                if (trimmed.length < 3) return 'At least 3 characters';
                if (!RegExp(r'^[a-zA-Z0-9_.]+$').hasMatch(trimmed)) {
                  return 'Only letters, numbers, _ and .';
                }
                if (_usernameStatus == _UsernameStatus.taken) {
                  return 'Username already taken';
                }
                return null;
              },
            ),
            const SizedBox(height: 16),

            _BioField(controller: _bioCtrl),
            const SizedBox(height: 16),

            _EditField(
              label: 'Website',
              controller: _websiteCtrl,
              keyboardType: TextInputType.url,
              validator: (v) {
                final trimmed = v?.trim() ?? '';
                if (trimmed.isEmpty) return null;
                final uri = Uri.tryParse(
                  trimmed.startsWith('http')
                      ? trimmed
                      : 'https://$trimmed',
                );
                if (uri == null || !uri.hasAuthority) {
                  return 'Enter a valid URL';
                }
                return null;
              },
            ),
            const SizedBox(height: 24),

            _PrivateToggle(
              value: _isPrivate,
              onChanged: (v) => setState(() => _isPrivate = v),
            ),
            const SizedBox(height: 40),
          ],
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Reusable field components
// ─────────────────────────────────────────────────────────────────────────────

class _EditField extends StatelessWidget {
  const _EditField({
    required this.label,
    required this.controller,
    this.maxLength,
    this.prefix,
    this.trailing,
    this.onChanged,
    this.validator,
    this.keyboardType,
  });

  final String label;
  final TextEditingController controller;
  final int? maxLength;
  final Widget? prefix;
  final Widget? trailing;
  final ValueChanged<String>? onChanged;
  final FormFieldValidator<String>? validator;
  final TextInputType? keyboardType;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(
              color: Colors.white54,
              fontSize: 12,
              fontWeight: FontWeight.w500),
        ),
        const SizedBox(height: 6),
        TextFormField(
          controller: controller,
          maxLength: maxLength,
          onChanged: onChanged,
          validator: validator,
          keyboardType: keyboardType,
          style: const TextStyle(color: Colors.white, fontSize: 15),
          decoration: InputDecoration(
            prefix: prefix,
            suffix: trailing,
            filled: true,
            fillColor: Colors.white.withValues(alpha: 0.07),
            counterText: '',
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: BorderSide.none,
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: const BorderSide(
                  color: Color(0xFF20D5EC), width: 1.5),
            ),
            errorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: const BorderSide(
                  color: Color(0xFFEE1D52), width: 1.5),
            ),
            focusedErrorBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: const BorderSide(
                  color: Color(0xFFEE1D52), width: 1.5),
            ),
            contentPadding: const EdgeInsets.symmetric(
                horizontal: 14, vertical: 12),
          ),
        ),
      ],
    );
  }
}

class _BioField extends StatefulWidget {
  const _BioField({required this.controller});

  final TextEditingController controller;

  @override
  State<_BioField> createState() => _BioFieldState();
}

class _BioFieldState extends State<_BioField> {
  static const int _maxLength = 150;
  int _charCount = 0;

  @override
  void initState() {
    super.initState();
    _charCount = widget.controller.text.length;
    widget.controller.addListener(_onChanged);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onChanged);
    super.dispose();
  }

  void _onChanged() {
    setState(() => _charCount = widget.controller.text.length);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Bio',
          style: TextStyle(
              color: Colors.white54,
              fontSize: 12,
              fontWeight: FontWeight.w500),
        ),
        const SizedBox(height: 6),
        Stack(
          children: [
            TextFormField(
              controller: widget.controller,
              maxLength: _maxLength,
              maxLines: 4,
              minLines: 3,
              style: const TextStyle(color: Colors.white, fontSize: 15),
              decoration: InputDecoration(
                filled: true,
                fillColor: Colors.white.withValues(alpha: 0.07),
                counterText: '',
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                  borderSide: BorderSide.none,
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                  borderSide: const BorderSide(
                      color: Color(0xFF20D5EC), width: 1.5),
                ),
                contentPadding:
                    const EdgeInsets.fromLTRB(14, 12, 14, 28),
              ),
            ),
            Positioned(
              bottom: 8,
              right: 12,
              child: Text(
                '$_charCount/$_maxLength',
                style: TextStyle(
                  color: _charCount >= _maxLength
                      ? const Color(0xFFEE1D52)
                      : Colors.white38,
                  fontSize: 11,
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _PrivateToggle extends StatelessWidget {
  const _PrivateToggle({
    required this.value,
    required this.onChanged,
  });

  final bool value;
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.07),
        borderRadius: BorderRadius.circular(8),
      ),
      child: SwitchListTile(
        contentPadding: EdgeInsets.zero,
        title: const Text(
          'Private account',
          style: TextStyle(
              color: Colors.white,
              fontSize: 15,
              fontWeight: FontWeight.w500),
        ),
        subtitle: Text(
          'Only approved followers can see your videos',
          style: TextStyle(
              color: Colors.white.withValues(alpha: 0.45),
              fontSize: 12),
        ),
        value: value,
        onChanged: onChanged,
        activeThumbColor: const Color(0xFFEE1D52),
      ),
    );
  }
}