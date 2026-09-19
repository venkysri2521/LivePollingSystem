package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"github.com/venkat/livepolls/backend/internal/db"
	"github.com/venkat/livepolls/backend/internal/middleware"
	"github.com/venkat/livepolls/backend/internal/models"
	"github.com/venkat/livepolls/backend/internal/validate"
)

// dummyHash lets the "no such user" branch spend the same time as a real
// password check, so response timing does not reveal which emails exist.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), 12)

type AuthHandler struct {
	Mongo *db.Mongo
	Auth  *middleware.Auth
}

type signupReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "send a valid JSON body")
		return
	}

	name, err := validate.Name(req.Name)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	email, err := validate.Email(req.Email)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := validate.Password(req.Password); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not create the account")
		return
	}

	user := models.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	res, err := h.Mongo.Users().InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			fail(c, http.StatusConflict, "an account with this email already exists")
			return
		}
		fail(c, http.StatusInternalServerError, "could not create the account")
		return
	}
	user.ID = res.InsertedID.(primitive.ObjectID)

	h.respondWithToken(c, http.StatusCreated, &user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "send a valid JSON body")
		return
	}
	email, err := validate.Email(req.Email)
	if err != nil {
		fail(c, http.StatusUnauthorized, "email or password is incorrect")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var user models.User
	err = h.Mongo.Users().FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Same message and a comparable amount of work either way, so the
			// response cannot be used to discover which emails are registered.
			bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
			fail(c, http.StatusUnauthorized, "email or password is incorrect")
			return
		}
		fail(c, http.StatusInternalServerError, "could not sign you in")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		fail(c, http.StatusUnauthorized, "email or password is incorrect")
		return
	}

	h.respondWithToken(c, http.StatusOK, &user)
}

func (h *AuthHandler) Me(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(middleware.UserID(c))
	if err != nil {
		fail(c, http.StatusUnauthorized, "sign in to continue")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var user models.User
	if err := h.Mongo.Users().FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		fail(c, http.StatusUnauthorized, "sign in to continue")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *AuthHandler) respondWithToken(c *gin.Context, status int, user *models.User) {
	token, exp, err := h.Auth.Issue(user.ID.Hex(), user.Name)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not start your session")
		return
	}
	c.JSON(status, gin.H{
		"token":     token,
		"expiresAt": exp,
		"user":      user,
	})
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}
