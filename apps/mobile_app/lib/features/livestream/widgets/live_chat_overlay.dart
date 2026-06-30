import 'dart:async';

import 'package:flutter/material.dart';

import '../models/livestream_model.dart';

/// Scrolling chat overlay rendered over the video player.
/// New messages appear at the bottom; the list auto-scrolls unless the
/// user has manually scrolled up.
class LiveChatOverlay extends StatefulWidget {
  const LiveChatOverlay({
    super.key,
    required this.messages,
    required this.pinnedMessage,
    required this.onSendMessage,
    required this.allowComments,
    this.currentUserId = '',
    this.onDeleteMessage,
    this.isHost = false,
  });

  final List<LiveMessage> messages;
  final LiveMessage? pinnedMessage;
  final Future<void> Function(String text) onSendMessage;
  final bool allowComments;
  final String currentUserId;
  final Future<void> Function(String messageId)? onDeleteMessage;
  final bool isHost;

  @override
  State<LiveChatOverlay> createState() => _LiveChatOverlayState();
}

class _LiveChatOverlayState extends State<LiveChatOverlay> {
  final ScrollController _scrollController = ScrollController();
  final TextEditingController _textController = TextEditingController();
  final FocusNode _focusNode = FocusNode();

  bool _atBottom = true;
  bool _sending = false;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void didUpdateWidget(LiveChatOverlay old) {
    super.didUpdateWidget(old);
    if (widget.messages.length != old.messages.length && _atBottom) {
      WidgetsBinding.instance.addPostFrameCallback((_) => _scrollToBottom());
    }
  }

  void _onScroll() {
    final pos = _scrollController.position;
    _atBottom = pos.pixels >= pos.maxScrollExtent - 40;
  }

  void _scrollToBottom() {
    if (_scrollController.hasClients) {
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOut,
      );
    }
  }

  Future<void> _sendMessage() async {
    final text = _textController.text.trim();
    if (text.isEmpty || _sending) return;
    setState(() => _sending = true);
    _textController.clear();
    try {
      await widget.onSendMessage(text);
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _textController.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.pinnedMessage != null) _buildPinnedMessage(),
        Flexible(
          child: _buildMessageList(),
        ),
        if (widget.allowComments) _buildInputRow(),
      ],
    );
  }

  Widget _buildPinnedMessage() {
    final msg = widget.pinnedMessage!;
    return Container(
      margin: const EdgeInsets.only(bottom: 4, left: 8, right: 8),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: Colors.amber.withValues(alpha: 0.8), width: 1),
      ),
      child: Row(
        children: [
          const Icon(Icons.push_pin, color: Colors.amber, size: 14),
          const SizedBox(width: 6),
          Expanded(
            child: RichText(
              text: TextSpan(
                children: [
                  TextSpan(
                    text: '${msg.username}: ',
                    style: const TextStyle(
                      color: Colors.amber,
                      fontWeight: FontWeight.bold,
                      fontSize: 12,
                    ),
                  ),
                  TextSpan(
                    text: msg.content,
                    style: const TextStyle(color: Colors.white, fontSize: 12),
                  ),
                ],
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMessageList() {
    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      itemCount: widget.messages.length,
      itemBuilder: (_, index) => _ChatBubble(
        message: widget.messages[index],
        currentUserId: widget.currentUserId,
        isHost: widget.isHost,
        onDelete: widget.onDeleteMessage,
      ),
    );
  }

  Widget _buildInputRow() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      child: Row(
        children: [
          Expanded(
            child: GestureDetector(
              onTap: () => FocusScope.of(context).requestFocus(_focusNode),
              child: Container(
                height: 36,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(18),
                ),
                child: TextField(
                  controller: _textController,
                  focusNode: _focusNode,
                  maxLength: 200,
                  maxLines: 1,
                  style: const TextStyle(color: Colors.white, fontSize: 13),
                  decoration: const InputDecoration(
                    hintText: 'Say something...',
                    hintStyle: TextStyle(color: Colors.white54, fontSize: 13),
                    border: InputBorder.none,
                    counterText: '',
                    contentPadding: EdgeInsets.symmetric(vertical: 8),
                  ),
                  onSubmitted: (_) => _sendMessage(),
                ),
              ),
            ),
          ),
          const SizedBox(width: 6),
          GestureDetector(
            onTap: _sendMessage,
            child: Container(
              width: 36,
              height: 36,
              decoration: const BoxDecoration(
                color: Color(0xFFFF2D55),
                shape: BoxShape.circle,
              ),
              child: _sending
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : const Icon(Icons.send, color: Colors.white, size: 16),
            ),
          ),
        ],
      ),
    );
  }
}

class _ChatBubble extends StatelessWidget {
  const _ChatBubble({
    required this.message,
    required this.currentUserId,
    required this.isHost,
    this.onDelete,
  });

  final LiveMessage message;
  final String currentUserId;
  final bool isHost;
  final Future<void> Function(String)? onDelete;

  bool get _isOwn => message.userId == currentUserId;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onLongPress: (isHost || _isOwn) && onDelete != null
          ? () => _showDeleteDialog(context)
          : null,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 2),
        child: RichText(
          text: TextSpan(
            children: [
              TextSpan(
                text: '${message.username} ',
                style: TextStyle(
                  color: _isOwn ? const Color(0xFFFF2D55) : Colors.white,
                  fontWeight: FontWeight.bold,
                  fontSize: 13,
                  shadows: const [Shadow(blurRadius: 2, color: Colors.black)],
                ),
              ),
              TextSpan(
                text: message.content,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 13,
                  shadows: [Shadow(blurRadius: 2, color: Colors.black)],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showDeleteDialog(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Delete message?',
            style: TextStyle(color: Colors.white)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              onDelete?.call(message.id);
            },
            child: const Text('Delete',
                style: TextStyle(color: Color(0xFFFF2D55))),
          ),
        ],
      ),
    );
  }
}
