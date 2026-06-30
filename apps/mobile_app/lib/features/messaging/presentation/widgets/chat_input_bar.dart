import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:image_picker/image_picker.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';

class ChatInputBar extends StatefulWidget {
  final void Function(String content, MessageType type, {String? replyToId, String? mediaUrl}) onSend;
  final void Function() onTyping;
  final void Function() onStopTyping;
  final MessageEntity? replyingTo;
  final VoidCallback? onCancelReply;

  const ChatInputBar({
    super.key,
    required this.onSend,
    required this.onTyping,
    required this.onStopTyping,
    this.replyingTo,
    this.onCancelReply,
  });

  @override
  State<ChatInputBar> createState() => _ChatInputBarState();
}

class _ChatInputBarState extends State<ChatInputBar>
    with SingleTickerProviderStateMixin {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();
  final _picker = ImagePicker();

  bool _hasText = false;
  bool _isRecording = false;
  bool _isCancelling = false;
  int _recordSeconds = 0;

  Timer? _typingTimer;
  Timer? _recordTimer;

  late final AnimationController _micAnim;
  late final Animation<double> _micScale;

  @override
  void initState() {
    super.initState();
    _controller.addListener(_onTextChanged);
    _micAnim = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    _micScale = Tween<double>(begin: 1.0, end: 1.15).animate(
      CurvedAnimation(parent: _micAnim, curve: Curves.easeInOut),
    );
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    _typingTimer?.cancel();
    _recordTimer?.cancel();
    _micAnim.dispose();
    super.dispose();
  }

  void _onTextChanged() {
    final has = _controller.text.trim().isNotEmpty;
    if (has != _hasText) setState(() => _hasText = has);

    // Debounced typing event.
    if (has) {
      widget.onTyping();
      _typingTimer?.cancel();
      _typingTimer = Timer(const Duration(seconds: 3), widget.onStopTyping);
    } else {
      _typingTimer?.cancel();
      widget.onStopTyping();
    }
  }

  void _sendText() {
    final text = _controller.text.trim();
    if (text.isEmpty) return;
    _controller.clear();
    _typingTimer?.cancel();
    widget.onStopTyping();
    widget.onSend(
      text,
      MessageType.text,
      replyToId: widget.replyingTo?.id,
    );
    widget.onCancelReply?.call();
  }

  Future<void> _pickMedia() async {
    final action = await showModalBottomSheet<String>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => const _MediaPickerSheet(),
    );
    if (action == null) return;

    XFile? file;
    if (action == 'image') {
      file = await _picker.pickImage(source: ImageSource.gallery, imageQuality: 85);
    } else if (action == 'video') {
      file = await _picker.pickVideo(source: ImageSource.gallery);
    } else if (action == 'camera') {
      file = await _picker.pickImage(source: ImageSource.camera, imageQuality: 85);
    }

    if (file == null || !mounted) return;

    // In a real app: upload via repository, get mediaUrl back, then send.
    // Here we send with the local path as placeholder.
    final type = action == 'video' ? MessageType.video : MessageType.image;
    widget.onSend('', type, mediaUrl: file.path);
  }

  void _startRecording() {
    HapticFeedback.mediumImpact();
    setState(() {
      _isRecording = true;
      _isCancelling = false;
      _recordSeconds = 0;
    });
    _micAnim.repeat(reverse: true);
    _recordTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() => _recordSeconds++);
    });
  }

  void _stopRecording({required bool cancel}) {
    _micAnim.stop();
    _micAnim.reset();
    _recordTimer?.cancel();
    final seconds = _recordSeconds;
    setState(() {
      _isRecording = false;
      _isCancelling = false;
    });
    if (!cancel && seconds >= 1) {
      // In production: save audio file, upload, then send.
      widget.onSend(
        '${seconds}s voice message',
        MessageType.voice,
      );
    }
  }

  void _showEmojiPicker() {
    // Emoji picker integration placeholder.
    // In production: use emoji_picker_flutter package.
    _focusNode.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (widget.replyingTo != null) _buildReplyBanner(),
          Container(
            decoration: const BoxDecoration(
              color: Color(0xFF0A0A0A),
              border: Border(
                top: BorderSide(color: Color(0xFF2A2A2A), width: 0.5),
              ),
            ),
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
            child: _isRecording ? _buildRecordingRow() : _buildInputRow(),
          ),
        ],
      ),
    );
  }

  Widget _buildReplyBanner() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      decoration: const BoxDecoration(
        color: Color(0xFF111111),
        border: Border(top: BorderSide(color: Color(0xFF2A2A2A), width: 0.5)),
      ),
      child: Row(
        children: [
          const Icon(Icons.reply, color: Color(0xFFFE2C55), size: 18),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              widget.replyingTo?.content ?? '',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: Colors.white54, fontSize: 13),
            ),
          ),
          GestureDetector(
            onTap: widget.onCancelReply,
            child: const Icon(Icons.close, color: Colors.white38, size: 18),
          ),
        ],
      ),
    );
  }

  Widget _buildInputRow() {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        // Emoji button
        IconButton(
          onPressed: _showEmojiPicker,
          icon: const Icon(Icons.emoji_emotions_outlined, color: Colors.white54),
          padding: const EdgeInsets.all(8),
          constraints: const BoxConstraints(),
        ),
        // Text field
        Expanded(
          child: Container(
            constraints: const BoxConstraints(maxHeight: 120),
            decoration: BoxDecoration(
              color: const Color(0xFF1A1A1A),
              borderRadius: BorderRadius.circular(24),
            ),
            child: TextField(
              controller: _controller,
              focusNode: _focusNode,
              maxLines: 5,
              minLines: 1,
              style: const TextStyle(color: Colors.white, fontSize: 15),
              decoration: const InputDecoration(
                hintText: 'Message...',
                hintStyle: TextStyle(color: Colors.white38),
                border: InputBorder.none,
                contentPadding:
                    EdgeInsets.symmetric(horizontal: 16, vertical: 10),
              ),
            ),
          ),
        ),
        const SizedBox(width: 4),
        // Attachment or send
        if (_hasText)
          _SendButton(onTap: _sendText)
        else ...[
          IconButton(
            onPressed: _pickMedia,
            icon: const Icon(Icons.attach_file, color: Colors.white54),
            padding: const EdgeInsets.all(8),
            constraints: const BoxConstraints(),
          ),
          // Mic — hold to record
          GestureDetector(
            onLongPressStart: (_) => _startRecording(),
            onLongPressEnd: (_) => _stopRecording(cancel: _isCancelling),
            onLongPressMoveUpdate: (details) {
              final cancel = details.offsetFromOrigin.dx < -60;
              if (cancel != _isCancelling) {
                setState(() => _isCancelling = cancel);
                if (cancel) HapticFeedback.lightImpact();
              }
            },
            child: ScaleTransition(
              scale: _micScale,
              child: Container(
                width: 40,
                height: 40,
                decoration: const BoxDecoration(
                  color: Color(0xFFFE2C55),
                  shape: BoxShape.circle,
                ),
                child: const Icon(Icons.mic, color: Colors.white, size: 20),
              ),
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildRecordingRow() {
    final mins = _recordSeconds ~/ 60;
    final secs = (_recordSeconds % 60).toString().padLeft(2, '0');
    final timeStr = '$mins:$secs';

    return Row(
      children: [
        // Cancel hint
        Expanded(
          child: Text(
            _isCancelling ? 'Release to cancel' : '← Slide to cancel',
            style: TextStyle(
              color: _isCancelling
                  ? const Color(0xFFFE2C55)
                  : Colors.white38,
              fontSize: 14,
            ),
          ),
        ),
        // Waveform bars animation
        _WaveformBars(active: !_isCancelling),
        const SizedBox(width: 8),
        // Timer
        Text(
          timeStr,
          style: const TextStyle(color: Colors.white70, fontSize: 14),
        ),
        const SizedBox(width: 12),
        // Mic indicator
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: _isCancelling
                ? Colors.white12
                : const Color(0xFFFE2C55),
            shape: BoxShape.circle,
          ),
          child: const Icon(Icons.mic, color: Colors.white, size: 20),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Send button
// ---------------------------------------------------------------------------

class _SendButton extends StatelessWidget {
  final VoidCallback onTap;
  const _SendButton({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 40,
        height: 40,
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            colors: [Color(0xFFFE2C55), Color(0xFFFF6B8A)],
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
          ),
          shape: BoxShape.circle,
        ),
        child: const Icon(Icons.send_rounded, color: Colors.white, size: 18),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Animated waveform bars (recording state)
// ---------------------------------------------------------------------------

class _WaveformBars extends StatefulWidget {
  final bool active;
  const _WaveformBars({required this.active});

  @override
  State<_WaveformBars> createState() => _WaveformBarsState();
}

class _WaveformBarsState extends State<_WaveformBars>
    with TickerProviderStateMixin {
  late final List<AnimationController> _bars;
  static const _count = 12;

  @override
  void initState() {
    super.initState();
    _bars = List.generate(_count, (i) {
      final c = AnimationController(
        vsync: this,
        duration: Duration(milliseconds: 300 + (i * 40)),
      );
      Future.delayed(Duration(milliseconds: i * 60), () {
        if (mounted) c.repeat(reverse: true);
      });
      return c;
    });
  }

  @override
  void dispose() {
    for (final c in _bars) { c.dispose(); }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      mainAxisSize: MainAxisSize.min,
      children: List.generate(_count, (i) {
        return AnimatedBuilder(
          animation: _bars[i],
          builder: (_, __) {
            final h = widget.active
                ? 4.0 + (_bars[i].value * 20.0)
                : 4.0;
            return Container(
              margin: const EdgeInsets.symmetric(horizontal: 1),
              width: 3,
              height: h,
              decoration: BoxDecoration(
                color: widget.active
                    ? const Color(0xFFFE2C55)
                    : Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            );
          },
        );
      }),
    );
  }
}

// ---------------------------------------------------------------------------
// Media picker sheet
// ---------------------------------------------------------------------------

class _MediaPickerSheet extends StatelessWidget {
  const _MediaPickerSheet();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.fromLTRB(16, 16, 16, 32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _SheetTile(
            icon: Icons.photo_library,
            label: 'Choose photo',
            value: 'image',
          ),
          _SheetTile(
            icon: Icons.videocam,
            label: 'Choose video',
            value: 'video',
          ),
          _SheetTile(
            icon: Icons.camera_alt,
            label: 'Take a photo',
            value: 'camera',
          ),
        ],
      ),
    );
  }
}

class _SheetTile extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;

  const _SheetTile({
    required this.icon,
    required this.label,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(icon, color: Colors.white70),
      title: Text(label, style: const TextStyle(color: Colors.white)),
      onTap: () => Navigator.pop(context, value),
    );
  }
}
