import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/repositories/message_repository.dart';

class SendMessageParams {
  final String conversationId;
  final String content;
  final MessageType type;
  final String? replyToId;
  final String? mediaUrl;

  const SendMessageParams({
    required this.conversationId,
    required this.content,
    required this.type,
    this.replyToId,
    this.mediaUrl,
  });
}

class SendMessageUseCase {
  final MessageRepository _repository;

  const SendMessageUseCase(this._repository);

  Future<Either<String, MessageEntity>> call(SendMessageParams params) {
    return _repository.sendMessage(
      params.conversationId,
      params.content,
      params.type,
      replyToId: params.replyToId,
      mediaUrl: params.mediaUrl,
    );
  }
}
