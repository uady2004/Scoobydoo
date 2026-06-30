import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/presentation/providers/messaging_provider.dart';
// import 'package:tiktok_clone/features/messaging/presentation/widgets/chat_input_bar.dart';
import 'package:tiktok_clone/features/messaging/presentation/widgets/message_bubble.dart';
import 'package:tiktok_clone/features/messaging/presentation/widgets/typing_indicator.dart';

class ChatScreen extends ConsumerStatefulWidget {
  final String conversationId;

  const ChatScreen({super.key, required this.conversationId});

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  bool _isMuted = false;
  final _scrollController = ScrollController();
  MessageEntity? _replyingTo;
  bool _showEmojiPicker = false;
  final _textController = TextEditingController();
  final _focusNode = FocusNode();

  // Emoji categories
  static const _emojiCategories = {
    '😀 Smileys': [
      '😀','😃','😄','😁','😆','😅','🤣','😂','🙂','🙃','😉','😊','😇',
      '🥰','😍','🤩','😘','😗','☺️','😚','😙','🥲','😋','😛','😜','🤪',
      '😝','🤑','🤗','🤭','🤫','🤔','🤐','🤨','😐','😑','😶','😏','😒',
      '🙄','😬','🤥','😌','😔','😪','🤤','😴','😷','🤒','🤕','🤢','🤮',
      '🤧','🥵','🥶','🥴','😵','🤯','🤠','🥳','🥸','😎','🤓','🧐','😕',
      '😟','🙁','☹️','😮','😯','😲','😳','🥺','😦','😧','😨','😰','😥',
      '😢','😭','😱','😖','😣','😞','😓','😩','😫','🥱','😤','😡','😠',
      '🤬','😈','👿','💀','☠️','💩','🤡','👹','👺','👻','👽','👾','🤖',
    ],
    '👍 Gestures': [
      '👋','🤚','🖐️','✋','🖖','👌','🤌','🤏','✌️','🤞','🤟','🤘','🤙',
      '👈','👉','👆','🖕','👇','☝️','👍','👎','✊','👊','🤛','🤜','👏',
      '🙌','👐','🤲','🤝','🙏','✍️','💅','🤳','💪','🦾','🦵','🦶','👂',
      '🦻','👃','🫀','🫁','🧠','🦷','🦴','👀','👁️','👅','👄','💋','🫦',
    ],
    '❤️ Hearts': [
      '❤️','🧡','💛','💚','💙','💜','🖤','🤍','🤎','💔','❣️','💕','💞',
      '💓','💗','💖','💘','💝','💟','☮️','✝️','☪️','🕉️','☸️','✡️',
      '🔯','🕎','☯️','☦️','🛐','⛎','♈','♉','♊','♋','♌','♍','♎','♏',
      '♐','♑','♒','♓','🆔','⚛️','🉑','☢️','☣️','📴','📳','🈶','🈚',
    ],
    '🎉 Celebration': [
      '🎉','🎊','🎈','🎁','🎀','🎗️','🎟️','🎫','🏆','🥇','🥈','🥉',
      '🏅','🎖️','🏵️','🎪','🤹','🎭','🎨','🖼️','🎬','🎤','🎧','🎼',
      '🎵','🎶','🎷','🪗','🎸','🎹','🎺','🎻','🪘','🥁','🪇','🎮','🕹️',
      '🎲','🎯','🎳','🎰','🎱','🔮','🪄','🧸','🪅','🎠','🎡','🎢',
    ],
    '🐶 Animals': [
      '🐶','🐱','🐭','🐹','🐰','🦊','🐻','🐼','🐻‍❄️','🐨','🐯','🦁',
      '🐮','🐷','🐸','🐵','🙈','🙉','🙊','🐒','🦆','🐧','🦅','🦉',
      '🦇','🐺','🐗','🐴','🦄','🐝','🪱','🐛','🦋','🐌','🐞','🐜',
      '🪲','🦟','🦗','🪳','🕷️','🦂','🐢','🐍','🦎','🦕','🦖','🐊',
    ],
    '🍕 Food': [
      '🍕','🍔','🍟','🌭','🍿','🧂','🥓','🥚','🍳','🧇','🥞','🧈',
      '🍞','🥐','🥖','🫓','🥨','🥯','🧀','🥗','🥙','🥪','🌮','🌯',
      '🫔','🍝','🍜','🍲','🍛','🍣','🍱','🥟','🦪','🍤','🍙','🍚',
      '🍘','🍥','🥮','🍢','🧁','🍰','🎂','🍮','🍭','🍬','🍫','🍿',
    ],
    '⚽ Sports': [
      '⚽','🏀','🏈','⚾','🥎','🎾','🏐','🏉','🥏','🎱','🏓','🏸',
      '🏒','🏑','🥍','🏏','🪃','🥅','⛳','🏹','🎣','🤿','🥊','🥋',
      '🎽','🛹','🛼','🛷','⛸️','🥌','🎿','⛷️','🏂','🪂','🏋️','🤼',
      '🤸','⛹️','🤺','🤾','🏌️','🏇','🧘','🏄','🏊','🤽','🚣','🧗',
    ],
    '🚀 Travel': [
      '🚀','✈️','🛸','🚁','🛶','⛵','🚢','🛥️','🚤','🛳️','⛴️','🚂',
      '🚃','🚄','🚅','🚆','🚇','🚈','🚉','🚊','🚝','🚞','🚋','🚌',
      '🚍','🚎','🚐','🚑','🚒','🚓','🚔','🚕','🚖','🚗','🚘','🚙',
      '🛻','🚚','🚛','🚜','🏎️','🏍️','🛵','🦽','🦼','🛺','🚲','🛴',
    ],
  };

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
    _focusNode.addListener(() {
      if (_focusNode.hasFocus && _showEmojiPicker) {
        setState(() => _showEmojiPicker = false);
      }
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(typingWatcherProvider(widget.conversationId));
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _textController.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      ref.read(chatProvider(widget.conversationId).notifier).loadMore();
    }
  }

  void _send(String content, MessageType type,
      {String? replyToId, String? mediaUrl}) {
    ref.read(chatProvider(widget.conversationId).notifier).sendMessage(
          content,
          type,
          replyToId: replyToId,
          replyToContent: _replyingTo?.content,
          mediaUrl: mediaUrl,
        );
    setState(() => _replyingTo = null);
  }

  void _onEmojiTap(String emoji) {
    final text = _textController.text;
    final selection = _textController.selection;
    final newText = text.replaceRange(
      selection.start < 0 ? text.length : selection.start,
      selection.end < 0 ? text.length : selection.end,
      emoji,
    );
    _textController.value = TextEditingValue(
      text: newText,
      selection: TextSelection.collapsed(
        offset: (selection.start < 0 ? text.length : selection.start) +
            emoji.length,
      ),
    );
  }

  void _toggleEmojiPicker() {
    if (_showEmojiPicker) {
      _focusNode.requestFocus();
      setState(() => _showEmojiPicker = false);
    } else {
      _focusNode.unfocus();
      setState(() => _showEmojiPicker = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final messagesAsync = ref.watch(chatProvider(widget.conversationId));
    final isTyping = ref.watch(typingProvider(widget.conversationId));
    final currentUserId = ref.watch(currentUserIdProvider);

    // Resolve conversation metadata from inbox cache
    final conversations = ref.watch(inboxProvider).valueOrNull ?? [];
    final conv = conversations.cast<dynamic>().firstWhere(
          (c) => c.id == widget.conversationId,
          orElse: () => null,
        );

    // Also check extra passed from navigation
    final extra = GoRouterState.of(context).extra as Map<String, dynamic>?;
    final displayName = conv?.displayName(currentUserId) ??
        extra?['displayName'] as String? ??
        'Chat';
    final avatarUrl = conv?.avatarUrl(currentUserId) as String? ??
        extra?['avatarUrl'] as String?;
    final isOnline = conv?.isOnline(currentUserId) as bool? ??
        extra?['isOnline'] as bool? ??
        false;

    // Get userId of the other participant for profile navigation
    final otherUserId = conv != null
        ? (conv.participants as List<Map<String, dynamic>>)
            .firstWhere(
              (p) => p['id'] != currentUserId,
              orElse: () => {},
            )['id'] as String? ?? ''
        : '';

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: _buildAppBar(
          displayName, avatarUrl, isOnline, otherUserId, context),
      body: Column(
        children: [
          // Messages list
          Expanded(
            child: messagesAsync.when(
              loading: () => const Center(
                child: CircularProgressIndicator(
                  valueColor:
                      AlwaysStoppedAnimation(Color(0xFFFE2C55)),
                  strokeWidth: 2,
                ),
              ),
              error: (err, _) => Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.error_outline,
                        color: Colors.white38, size: 48),
                    const SizedBox(height: 12),
                    Text(
                      err.toString(),
                      style: const TextStyle(
                          color: Colors.white38, fontSize: 14),
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 16),
                    TextButton(
                      onPressed: () => ref
                          .invalidate(chatProvider(widget.conversationId)),
                      child: const Text('Try again',
                          style: TextStyle(color: Color(0xFFFE2C55))),
                    ),
                  ],
                ),
              ),
              data: (messages) {
                if (messages.isEmpty) {
                  return const _EmptyChatState();
                }
                return ListView.builder(
                  controller: _scrollController,
                  reverse: true,
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  itemCount: messages.length + (isTyping ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (isTyping && index == 0) {
                      return const TypingIndicator();
                    }
                    final msgIndex = isTyping ? index - 1 : index;
                    final message = messages[msgIndex];
                    final isOwn = message.senderId == currentUserId;
                    final isLastInGroup =
                        msgIndex == messages.length - 1 ||
                            messages[msgIndex + 1].senderId !=
                                message.senderId;

                    return MessageBubble(
                      key: ValueKey(message.id),
                      message: message,
                      isOwn: isOwn,
                      currentUserId: currentUserId,
                      showAvatar: !isOwn && isLastInGroup,
                      onReply: () =>
                          setState(() => _replyingTo = message),
                      onDelete: () {
                        ref
                            .read(chatProvider(widget.conversationId)
                                .notifier)
                            .deleteMessage(message.id);
                      },
                      onForward: () {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(
                            content: Text(
                                'Select a conversation to forward to'),
                            backgroundColor: Color(0xFF1A1A1A),
                            behavior: SnackBarBehavior.floating,
                          ),
                        );
                      },
                    );            // closes MessageBubble
                  },              // closes itemBuilder
                );                // closes ListView.builder
              },                  // closes data: (messages)
            ),                    // closes .when(
          ),                      // closes Expanded                // closes Expanded
       
          // Reply bar
          if (_replyingTo != null)
            Container(
              color: const Color(0xFF1A1A1A),
              padding: const EdgeInsets.symmetric(
                  horizontal: 12, vertical: 8),
              child: Row(
                children: [
                  const Icon(Icons.reply,
                      color: Color(0xFFFE2C55), size: 18),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      _replyingTo!.content,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                          color: Colors.white54, fontSize: 13),
                    ),
                  ),
                  GestureDetector(
                    onTap: () => setState(() => _replyingTo = null),
                    child: const Icon(Icons.close,
                        color: Colors.white38, size: 18),
                  ),
                ],
              ),
            ),

          // Input bar
          _ChatInputRow(
            controller: _textController,
            focusNode: _focusNode,
            showEmojiPicker: _showEmojiPicker,
            onToggleEmoji: _toggleEmojiPicker,
            onSend: () {
              final text = _textController.text.trim();
              if (text.isEmpty) return;
              _send(
                text,
                MessageType.text,
                replyToId: _replyingTo?.id,
              );
              _textController.clear();
            },
            onTyping: () => ref
                .read(chatProvider(widget.conversationId).notifier)
                .sendTypingEvent(),
            onStopTyping: () => ref
                .read(chatProvider(widget.conversationId).notifier)
                .sendStopTypingEvent(),
          ),

          // Emoji picker
          if (_showEmojiPicker)
            _EmojiPicker(onEmojiTap: _onEmojiTap),
        ],
      ),
    );
  }

  PreferredSizeWidget _buildAppBar(
    String displayName,
    String? avatarUrl,
    bool isOnline,
    String otherUserId,
    BuildContext context,
  ) {
    return AppBar(
      backgroundColor: Colors.black,
      elevation: 0,
      leadingWidth: 40,
      leading: IconButton(
        icon: const Icon(Icons.arrow_back_ios_new,
            color: Colors.white, size: 18),
        onPressed: () => context.pop(),
      ),
      title: GestureDetector(
        // ── Tap profile to navigate ──────────────────────────────────
        onTap: () {
          if (otherUserId.isNotEmpty) {
            context.push('/profile/$otherUserId');
          }
        },
        child: Row(
          children: [
            Stack(
              children: [
                CircleAvatar(
                  radius: 18,
                  backgroundColor: const Color(0xFF2A2A2A),
                  backgroundImage: avatarUrl != null
                      ? CachedNetworkImageProvider(avatarUrl)
                      : null,
                  child: avatarUrl == null
                      ? Text(
                          displayName.isNotEmpty
                              ? displayName[0].toUpperCase()
                              : '?',
                          style: const TextStyle(
                              color: Colors.white,
                              fontSize: 14,
                              fontWeight: FontWeight.w600),
                        )
                      : null,
                ),
                if (isOnline)
                  Positioned(
                    right: 1,
                    bottom: 1,
                    child: Container(
                      width: 9,
                      height: 9,
                      decoration: BoxDecoration(
                        color: const Color(0xFF00D68F),
                        shape: BoxShape.circle,
                        border: Border.all(
                            color: Colors.black, width: 1.5),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 10),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  displayName,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  isOnline ? 'Active now' : 'Offline',
                  style: TextStyle(
                    color: isOnline
                        ? const Color(0xFF00D68F)
                        : Colors.white38,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
      actions: [
        if (_isMuted)
          const Padding(
            padding: EdgeInsets.only(right: 4),
            child: Icon(Icons.notifications_off,
                color: Colors.white38, size: 18),
          ),
        IconButton(
          icon: const Icon(Icons.videocam_outlined,
              color: Colors.white, size: 24),
          onPressed: () {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(
                content: Text('Video call coming soon'),
                backgroundColor: Color(0xFF1A1A1A),
                behavior: SnackBarBehavior.floating,
              ),
            );
          },
        ),
        IconButton(
          icon: const Icon(Icons.more_vert,
              color: Colors.white, size: 22),
          onPressed: () => _showMoreSheet(context, otherUserId),
        ),
      ],
    );
  }

  void _showMoreSheet(BuildContext context, String otherUserId) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.of(context).viewInsets.bottom,
        ),
        child: SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 8),
              Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: Colors.white24,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: 8),
              ListTile(
                leading: const Icon(Icons.person_outline,
                    color: Colors.white70),
                title: const Text('View profile',
                    style: TextStyle(color: Colors.white)),
                onTap: () {
                  Navigator.pop(context);
                  if (otherUserId.isNotEmpty) {
                    context.push('/profile/$otherUserId');
                  }
                },
              ),
              ListTile(
                leading: Icon(
                  _isMuted
                      ? Icons.notifications_active_outlined
                      : Icons.notifications_off_outlined,
                  color: Colors.white70,
                ),
                title: Text(
                  _isMuted
                      ? 'Unmute notifications'
                      : 'Mute notifications',
                  style: const TextStyle(color: Colors.white),
                ),
                onTap: () {
                  Navigator.pop(context);
                  setState(() => _isMuted = !_isMuted);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(
                        _isMuted
                            ? 'Notifications muted'
                            : 'Notifications unmuted',
                      ),
                      backgroundColor: const Color(0xFF1A1A1A),
                      behavior: SnackBarBehavior.floating,
                    ),
                  );
                },
              ),
              ListTile(
                leading: const Icon(Icons.delete_outline,
                    color: Colors.white70),
                title: const Text('Delete conversation',
                    style: TextStyle(color: Colors.white)),
                onTap: () {
                  Navigator.pop(context);
                  _confirmDeleteConversation(context);
                },
              ),
              ListTile(
                leading: const Icon(Icons.block,
                    color: Color(0xFFEE1D52)),
                title: const Text('Block user',
                    style: TextStyle(color: Color(0xFFEE1D52))),
                onTap: () => Navigator.pop(context),
              ),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }

  void _confirmDeleteConversation(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Delete conversation',
            style: TextStyle(color: Colors.white)),
        content: const Text(
          'This will permanently delete all messages in this conversation.',
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
              context.pop(); // Go back to inbox
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text('Conversation deleted'),
                  backgroundColor: Color(0xFF1A1A1A),
                  behavior: SnackBarBehavior.floating,
                ),
              );
            },
            child: const Text('Delete',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Chat input row
// ---------------------------------------------------------------------------

class _ChatInputRow extends StatefulWidget {
  final TextEditingController controller;
  final FocusNode focusNode;
  final bool showEmojiPicker;
  final VoidCallback onToggleEmoji;
  final VoidCallback onSend;
  final VoidCallback onTyping;
  final VoidCallback onStopTyping;

  const _ChatInputRow({
    required this.controller,
    required this.focusNode,
    required this.showEmojiPicker,
    required this.onToggleEmoji,
    required this.onSend,
    required this.onTyping,
    required this.onStopTyping,
  });

  @override
  State<_ChatInputRow> createState() => _ChatInputRowState();
}

class _ChatInputRowState extends State<_ChatInputRow> {
  bool _hasText = false;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(() {
      final has = widget.controller.text.trim().isNotEmpty;
      if (has != _hasText) setState(() => _hasText = has);
      if (has) {
        widget.onTyping();
      } else {
        widget.onStopTyping();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFF0A0A0A),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
      child: Row(
        children: [
          // Emoji toggle
          GestureDetector(
            onTap: widget.onToggleEmoji,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 6),
              child: Icon(
                widget.showEmojiPicker
                    ? Icons.keyboard_alt_outlined
                    : Icons.emoji_emotions_outlined,
                color: Colors.white54,
                size: 26,
              ),
            ),
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
                controller: widget.controller,
                focusNode: widget.focusNode,
                style: const TextStyle(
                    color: Colors.white, fontSize: 15),
                maxLines: null,
                textInputAction: TextInputAction.newline,
                decoration: const InputDecoration(
                  hintText: 'Message...',
                  hintStyle: TextStyle(color: Colors.white38),
                  contentPadding: EdgeInsets.symmetric(
                      horizontal: 16, vertical: 10),
                  border: InputBorder.none,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          // Send / mic button
          GestureDetector(
            onTap: widget.onSend,
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: _hasText
                    ? const Color(0xFFFE2C55)
                    : const Color(0xFF1A1A1A),
                shape: BoxShape.circle,
              ),
              child: Icon(
                _hasText ? Icons.send_rounded : Icons.mic_outlined,
                color: _hasText ? Colors.white : Colors.white54,
                size: 20,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Emoji picker
// ---------------------------------------------------------------------------

class _EmojiPicker extends StatefulWidget {
  final ValueChanged<String> onEmojiTap;

  const _EmojiPicker({required this.onEmojiTap});

  @override
  State<_EmojiPicker> createState() => _EmojiPickerState();
}

class _EmojiPickerState extends State<_EmojiPicker>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  static const _emojiCategories = {
    '😀': [
      '😀','😃','😄','😁','😆','😅','🤣','😂','🙂','🙃','😉','😊','😇',
      '🥰','😍','🤩','😘','😗','☺️','😚','😙','🥲','😋','😛','😜','🤪',
      '😝','🤑','🤗','🤭','🤫','🤔','🤐','🤨','😐','😑','😶','😏','😒',
      '🙄','😬','🤥','😌','😔','😪','🤤','😴','😷','🤒','🤕','🤢','🤮',
      '🤧','🥵','🥶','🥴','😵','🤯','🤠','🥳','😎','🤓','🧐','😕','😟',
      '🙁','☹️','😮','😯','😲','😳','🥺','😦','😧','😨','😰','😥','😢',
      '😭','😱','😖','😣','😞','😓','😩','😫','🥱','😤','😡','😠','🤬',
      '😈','👿','💀','☠️','💩','🤡','👹','👺','👻','👽','👾','🤖',
    ],
    '👋': [
      '👋','🤚','🖐️','✋','🖖','👌','🤌','🤏','✌️','🤞','🤟','🤘','🤙',
      '👈','👉','👆','🖕','👇','☝️','👍','👎','✊','👊','🤛','🤜','👏',
      '🙌','👐','🤲','🤝','🙏','✍️','💅','🤳','💪','🦾','🦵','🦶','👂',
      '🦻','👃','🧠','🦷','🦴','👀','👁️','👅','👄','💋','🫦',
    ],
    '❤️': [
      '❤️','🧡','💛','💚','💙','💜','🖤','🤍','🤎','💔','❣️','💕','💞',
      '💓','💗','💖','💘','💝','💟','♥️','🔴','🟠','🟡','🟢','🔵','🟣',
      '⚫','⚪','🟤','❤️‍🔥','❤️‍🩹','💯','💢','💥','💫','💦','💨','🕳️',
    ],
    '🐶': [
      '🐶','🐱','🐭','🐹','🐰','🦊','🐻','🐼','🐨','🐯','🦁','🐮',
      '🐷','🐸','🐵','🙈','🙉','🙊','🐒','🦆','🐧','🦅','🦉','🦇',
      '🐺','🐗','🐴','🦄','🐝','🦋','🐌','🐞','🐜','🐢','🐍','🦎',
      '🐊','🦕','🦖','🦈','🐳','🐋','🐬','🦭','🐟','🐠','🐡','🦐',
    ],
    '🍕': [
      '🍕','🍔','🍟','🌭','🍿','🥓','🥚','🍳','🧇','🥞','🧈','🍞',
      '🥐','🥖','🫓','🥨','🥯','🧀','🥗','🥙','🥪','🌮','🌯','🍝',
      '🍜','🍲','🍛','🍣','🍱','🥟','🦪','🍤','🍙','🍚','🍘','🍥',
      '🥮','🍢','🧁','🍰','🎂','🍮','🍭','🍬','🍫','🍩','🍪','🍦',
    ],
    '⚽': [
      '⚽','🏀','🏈','⚾','🥎','🎾','🏐','🏉','🥏','🎱','🏓','🏸',
      '🏒','🏑','🥍','🏏','🪃','🥅','⛳','🏹','🎣','🤿','🥊','🥋',
      '🎽','🛹','🛼','🛷','⛸️','🥌','🎿','⛷️','🏂','🪂','🏋️',
      '🤼','🤸','⛹️','🤺','🤾','🏌️','🏇','🧘','🏄','🏊','🤽','🚣',
    ],
    '🚀': [
      '🚀','✈️','🛸','🚁','🛶','⛵','🚢','🛥️','🚤','🛳️','🚂','🚃',
      '🚄','🚅','🚆','🚇','🚈','🚉','🚊','🚝','🚞','🚋','🚌','🚍',
      '🚎','🚐','🚑','🚒','🚓','🚔','🚕','🚖','🚗','🚘','🚙','🛻',
      '🚚','🚛','🚜','🏎️','🏍️','🛵','🚲','🛴','🛺','🚨','🚥','🚦',
    ],
    '💡': [
      '⌚','📱','💻','⌨️','🖥️','🖨️','🖱️','🖲️','💽','💾','💿','📀',
      '📷','📸','📹','🎥','📽️','🎞️','📞','☎️','📟','📠','📺','📻',
      '🧭','⏱️','⏲️','⏰','🕰️','⌛','⏳','📡','🔋','🪫','🔌','💡',
      '🔦','🕯️','🪔','🧯','🛢️','💸','💵','💴','💶','💷','🪙','💳',
    ],
  };

  @override
  void initState() {
    super.initState();
    _tabController =
        TabController(length: _emojiCategories.length, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 280,
      color: const Color(0xFF0A0A0A),
      child: Column(
        children: [
          // Category tabs
          TabBar(
            controller: _tabController,
            isScrollable: true,
            indicatorColor: const Color(0xFFFE2C55),
            indicatorSize: TabBarIndicatorSize.label,
            tabs: _emojiCategories.keys
                .map((icon) => Tab(
                      child: Text(icon,
                          style: const TextStyle(fontSize: 20)),
                    ))
                .toList(),
          ),
          // Emoji grid
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: _emojiCategories.values.map((emojis) {
                return GridView.builder(
                  padding: const EdgeInsets.all(8),
                  gridDelegate:
                      const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 8,
                    mainAxisSpacing: 4,
                    crossAxisSpacing: 4,
                  ),
                  itemCount: emojis.length,
                  itemBuilder: (context, index) {
                    return GestureDetector(
                      onTap: () => widget.onEmojiTap(emojis[index]),
                      child: Center(
                        child: Text(
                          emojis[index],
                          style: const TextStyle(fontSize: 24),
                        ),
                      ),
                    );
                  },
                );
              }).toList(),
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Empty chat placeholder
// ---------------------------------------------------------------------------

class _EmptyChatState extends StatelessWidget {
  const _EmptyChatState();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text('👋', style: TextStyle(fontSize: 48)),
          SizedBox(height: 12),
          Text(
            'Say hello!',
            style: TextStyle(color: Colors.white54, fontSize: 16),
          ),
        ],
      ),
    );
  }
}