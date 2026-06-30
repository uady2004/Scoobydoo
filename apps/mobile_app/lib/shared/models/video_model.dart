class VideoModel{

final int id;

final String url;

final String description;

final int likes;

VideoModel({

required this.id,

required this.url,

required this.description,

required this.likes,

});

factory VideoModel.fromJson(
Map<String,dynamic> json
){

return VideoModel(

id:json["id"],

url:json["url"]??"",

description:
json["description"]??"",

likes:
json["likes"]??0,

);

}

}